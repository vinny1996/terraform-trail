// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package local

import (
	"context"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"sort"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/terraform/internal/addrs"
	"github.com/hashicorp/terraform/internal/backend"
	"github.com/hashicorp/terraform/internal/command/views"
	"github.com/hashicorp/terraform/internal/configs"
	"github.com/hashicorp/terraform/internal/lang/marks"
	"github.com/hashicorp/terraform/internal/logging"
	"github.com/hashicorp/terraform/internal/moduletest"
	configtest "github.com/hashicorp/terraform/internal/moduletest/config"
	hcltest "github.com/hashicorp/terraform/internal/moduletest/hcl"
	"github.com/hashicorp/terraform/internal/moduletest/mocking"
	"github.com/hashicorp/terraform/internal/plans"
	"github.com/hashicorp/terraform/internal/states"
	"github.com/hashicorp/terraform/internal/terraform"
	"github.com/hashicorp/terraform/internal/tfdiags"
)

const (
	MainStateIdentifier = ""
)

type TestSuiteRunner struct {
	Config *configs.Config

	TestingDirectory string

	// Global variables comes from the main configuration directory,
	// and the Global Test Variables are loaded from the test director.
	GlobalVariables map[string]backend.UnparsedVariableValue
	GlobalTestVariables map[string]backend.UnparsedVariableValue

	Opts            *terraform.ContextOpts

	View views.Test

	// Stopped and Cancelled track whether the user requested the testing
	// process to be interrupted. Stopped is a nice graceful exit, we'll still
	// tidy up any state that was created and mark the tests with relevant
	// `skipped` status updates. Cancelled is a hard stop right now exit, we
	// won't attempt to clean up any state left hanging, and tests will just
	// be left showing `pending` as the status. We will still print out the
	// destroy summary diagnostics that tell the user what state has been left
	// behind and needs manual clean up.
	Stopped   bool
	Cancelled bool

	// StoppedCtx and CancelledCtx allow in progress Terraform operations to
	// respond to external calls from the test command.
	StoppedCtx   context.Context
	CancelledCtx context.Context

	// Filter restricts exactly which test files will be executed.
	Filter []string

	// Verbose tells the runner to print out plan files during each test run.
	Verbose bool

	// configProviders is a cache of config keys mapped to all the providers
	// referenced by the given config.
	//
	// The config keys are globally unique across an entire test suite, so we
	// store this at the suite runner level to get maximum efficiency.
	configProviders map[string]map[string]bool
}

func (runner *TestSuiteRunner) Stop() {
	runner.Stopped = true
}

func (runner *TestSuiteRunner) Cancel() {
	runner.Cancelled = true
}

func (runner *TestSuiteRunner) Test() (moduletest.Status, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics

	// First thing, initialise the config providers map.
	runner.configProviders = make(map[string]map[string]bool)

	suite, suiteDiags := runner.collectTests()
	diags = diags.Append(suiteDiags)
	if suiteDiags.HasErrors() {
		return moduletest.Error, diags
	}

	runner.View.Abstract(suite)

	var files []string
	for name := range suite.Files {
		files = append(files, name)
	}
	sort.Strings(files) // execute the files in alphabetical order

	suite.Status = moduletest.Pass
	for _, name := range files {
		if runner.Cancelled {
			return suite.Status, diags
		}

		file := suite.Files[name]

		priorStates := make(map[string]*terraform.TestContext)
		for _, run := range file.Runs {
			// Pre-initialise the prior states, so we can easily tell between
			// a run block that doesn't exist and a run block that hasn't been
			// executed yet.
			priorStates[run.Name] = nil
		}

		fileRunner := &TestFileRunner{
			Suite: runner,
			RelevantStates: map[string]*TestFileState{
				MainStateIdentifier: {
					Run:   nil,
					State: states.NewState(),
				},
			},
			PriorStates: priorStates,
		}

		runner.View.File(file, moduletest.Starting)
		fileRunner.Test(file)
		runner.View.File(file, moduletest.TearDown)
		fileRunner.cleanup(file)
		runner.View.File(file, moduletest.Complete)
		suite.Status = suite.Status.Merge(file.Status)
	}

	runner.View.Conclusion(suite)

	return suite.Status, diags
}

func (runner *TestSuiteRunner) collectTests() (*moduletest.Suite, tfdiags.Diagnostics) {
	runCount := 0
	fileCount := 0

	var diags tfdiags.Diagnostics
	suite := &moduletest.Suite{
		Files: func() map[string]*moduletest.File {
			files := make(map[string]*moduletest.File)

			if len(runner.Filter) > 0 {
				for _, name := range runner.Filter {
					file, ok := runner.Config.Module.Tests[name]
					if !ok {
						// If the filter is invalid, we'll simply skip this
						// entry and print a warning. But we could still execute
						// any other tests within the filter.
						diags.Append(tfdiags.Sourceless(
							tfdiags.Warning,
							"Unknown test file",
							fmt.Sprintf("The specified test file, %s, could not be found.", name)))
						continue
					}

					fileCount++

					var runs []*moduletest.Run
					for ix, run := range file.Runs {
						runs = append(runs, &moduletest.Run{
							Config: run,
							Index:  ix,
							Name:   run.Name,
						})
					}

					runCount += len(runs)
					files[name] = &moduletest.File{
						Config: file,
						Name:   name,
						Runs:   runs,
					}
				}

				return files
			}

			// Otherwise, we'll just do all the tests in the directory!
			for name, file := range runner.Config.Module.Tests {
				fileCount++

				var runs []*moduletest.Run
				for ix, run := range file.Runs {
					runs = append(runs, &moduletest.Run{
						Config: run,
						Index:  ix,
						Name:   run.Name,
					})
				}

				runCount += len(runs)
				files[name] = &moduletest.File{
					Config: file,
					Name:   name,
					Runs:   runs,
				}
			}
			return files
		}(),
	}

	log.Printf("[DEBUG] TestSuiteRunner: found %d files with %d run blocks", fileCount, runCount)

	return suite, diags
}

type TestFileRunner struct {
	// Suite contains all the helpful metadata about the test that we need
	// during the execution of a file.
	Suite *TestSuiteRunner

	// RelevantStates is a mapping of module keys to it's last applied state
	// file.
	//
	// This is used to clean up the infrastructure created during the test after
	// the test has finished.
	RelevantStates map[string]*TestFileState

	// PriorStates is mapping from run block names to the TestContexts that were
	// created when that run block executed.
	//
	// This is used to allow run blocks to refer back to the output values of
	// previous run blocks. It is passed into the Evaluate functions that
	// validate the test assertions, and used when calculating values for
	// variables within run blocks.
	PriorStates map[string]*terraform.TestContext

	globalVariables map[string]backend.UnparsedVariableValue
}

// TestFileState is a helper struct that just maps a run block to the state that
// was produced by the execution of that run block.
type TestFileState struct {
	Run   *moduletest.Run
	State *states.State
}

func (runner *TestFileRunner) Test(file *moduletest.File) {
	log.Printf("[TRACE] TestFileRunner: executing test file %s", file.Name)

	// First thing, initialise the global variables for the file
	runner.initVariables(file)

	// The file validation only returns warnings so we'll just add them without
	// checking anything about them.
	file.Diagnostics = file.Diagnostics.Append(file.Config.Validate(runner.Suite.Config))

	// We'll execute the tests in the file. First, mark the overall status as
	// being skipped. This will ensure that if we've cancelled and the files not
	// going to do anything it'll be marked as skipped.
	file.Status = file.Status.Merge(moduletest.Skip)
	if len(file.Runs) == 0 {
		// If we have zero run blocks then we'll just mark the file as passed.
		file.Status = file.Status.Merge(moduletest.Pass)
	}

	// Now execute the runs.
	for _, run := range file.Runs {
		if runner.Suite.Cancelled {
			// This means a hard stop has been requested, in this case we don't
			// even stop to mark future tests as having been skipped. They'll
			// just show up as pending in the printed summary. We will quickly
			// just mark the overall file status has having errored to indicate
			// it was interrupted.
			file.Status = file.Status.Merge(moduletest.Error)
			return
		}

		if runner.Suite.Stopped {
			// Then the test was requested to be stopped, so we just mark each
			// following test as skipped, print the status, and move on.
			run.Status = moduletest.Skip
			runner.Suite.View.Run(run, file, moduletest.Complete, 0)
			continue
		}

		if file.Status == moduletest.Error {
			// If the overall test file has errored, we don't keep trying to
			// execute tests. Instead, we mark all remaining run blocks as
			// skipped, print the status, and move on.
			run.Status = moduletest.Skip
			runner.Suite.View.Run(run, file, moduletest.Complete, 0)
			continue
		}

		key := MainStateIdentifier
		config := runner.Suite.Config
		if run.Config.ConfigUnderTest != nil {
			config = run.Config.ConfigUnderTest
			// Then we need to load an alternate state and not the main one.

			key = run.Config.Module.Source.String()
			if key == MainStateIdentifier {
				// This is bad. It means somehow the module we're loading has
				// the same key as main state and we're about to corrupt things.

				run.Diagnostics = run.Diagnostics.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid module source",
					Detail:   fmt.Sprintf("The source for the selected module evaluated to %s which should not be possible. This is a bug in Terraform - please report it!", key),
					Subject:  run.Config.Module.DeclRange.Ptr(),
				})

				run.Status = moduletest.Error
				file.Status = moduletest.Error
				continue // Abort!
			}

			if _, exists := runner.RelevantStates[key]; !exists {
				runner.RelevantStates[key] = &TestFileState{
					Run:   nil,
					State: states.NewState(),
				}
			}
		}

		state, updatedState := runner.run(run, file, runner.RelevantStates[key].State, config)
		if updatedState {
			// Only update the most recent run and state if the state was
			// actually updated by this change. We want to use the run that
			// most recently updated the tracked state as the cleanup
			// configuration.
			runner.RelevantStates[key].State = state
			runner.RelevantStates[key].Run = run
		}

		runner.Suite.View.Run(run, file, moduletest.Complete, 0)
		file.Status = file.Status.Merge(run.Status)
	}
}

func (runner *TestFileRunner) run(run *moduletest.Run, file *moduletest.File, state *states.State, config *configs.Config) (*states.State, bool) {
	log.Printf("[TRACE] TestFileRunner: executing run block %s/%s", file.Name, run.Name)

	if runner.Suite.Cancelled {
		// Don't do anything, just give up and return immediately.
		// The surrounding functions should stop this even being called, but in
		// case of race conditions or something we can still verify this.
		return state, false
	}

	if runner.Suite.Stopped {
		// Basically the same as above, except we'll be a bit nicer.
		run.Status = moduletest.Skip
		return state, false
	}

	start := time.Now().UTC().UnixMilli()
	runner.Suite.View.Run(run, file, moduletest.Starting, 0)

	run.Diagnostics = run.Diagnostics.Append(run.Config.Validate(config))
	if run.Diagnostics.HasErrors() {
		run.Status = moduletest.Error
		return state, false
	}

	key := MainStateIdentifier
	if run.Config.ConfigUnderTest != nil {
		key = run.Config.Module.Source.String()
	}
	runner.gatherProviders(key, config)

	resetConfig, configDiags := configtest.TransformConfigForTest(config, run, file, runner.globalVariables, runner.PriorStates, runner.Suite.configProviders[key])
	defer resetConfig()

	run.Diagnostics = run.Diagnostics.Append(configDiags)
	if configDiags.HasErrors() {
		run.Status = moduletest.Error
		return state, false
	}

	validateDiags := runner.validate(config, run, file, start)
	run.Diagnostics = run.Diagnostics.Append(validateDiags)
	if validateDiags.HasErrors() {
		run.Status = moduletest.Error
		return state, false
	}

	references, referenceDiags := run.GetReferences()
	run.Diagnostics = run.Diagnostics.Append(referenceDiags)
	if referenceDiags.HasErrors() {
		run.Status = moduletest.Error
		return state, false
	}

	variables, variableDiags := runner.GetVariables(config, run, references)
	run.Diagnostics = run.Diagnostics.Append(variableDiags)
	if variableDiags.HasErrors() {
		run.Status = moduletest.Error
		return state, false
	}

	// FilterVariablesToConfig only returns warnings, so we don't check the
	// returned diags for errors.
	setVariables, setVariableDiags := runner.FilterVariablesToConfig(config, variables)
	run.Diagnostics = run.Diagnostics.Append(setVariableDiags)

	planCtx, plan, planDiags := runner.plan(config, state, run, file, setVariables, references, start)
	if run.Config.Command == configs.PlanTestCommand {
		// Then we want to assess our conditions and diagnostics differently.
		planDiags = run.ValidateExpectedFailures(planDiags)
		run.Diagnostics = run.Diagnostics.Append(planDiags)
		if planDiags.HasErrors() {
			run.Status = moduletest.Error
			return state, false
		}

		resetVariables := runner.AddVariablesToConfig(config, variables)
		defer resetVariables()

		if runner.Suite.Verbose {
			schemas, diags := planCtx.Schemas(config, plan.PlannedState)

			// If we're going to fail to render the plan, let's not fail the overall
			// test. It can still have succeeded. So we'll add the diagnostics, but
			// still report the test status as a success.
			if diags.HasErrors() {
				// This is very unlikely.
				diags = diags.Append(tfdiags.Sourceless(
					tfdiags.Warning,
					"Failed to print verbose output",
					fmt.Sprintf("Terraform failed to print the verbose output for %s, other diagnostics will contain more details as to why.", path.Join(file.Name, run.Name))))
			} else {
				run.Verbose = &moduletest.Verbose{
					Plan:         plan,
					State:        plan.PlannedState,
					Config:       config,
					Providers:    schemas.Providers,
					Provisioners: schemas.Provisioners,
				}
			}

			run.Diagnostics = run.Diagnostics.Append(diags)
		}

		// First, make the test context we can use to validate the assertions
		// of the
		ctx := planCtx.TestContext(run, config, plan.PlannedState, plan, variables)

		// Second, evaluate the run block directly. We also pass in all the
		// previous contexts so this run block can refer to outputs from
		// previous run blocks.
		ctx.Evaluate(runner.PriorStates)

		// Now we've successfully validated this run block, lets add it into
		// our prior states so future run blocks can access it.
		runner.PriorStates[run.Name] = ctx

		return state, false
	}

	// Otherwise any error during the planning prevents our apply from
	// continuing which is an error.
	planDiags = run.ExplainExpectedFailures(planDiags)
	run.Diagnostics = run.Diagnostics.Append(planDiags)
	if planDiags.HasErrors() {
		run.Status = moduletest.Error
		return state, false
	}

	// Since we're carrying on an executing the apply operation as well, we're
	// just going to do some post processing of the diagnostics. We remove the
	// warnings generated from check blocks, as the apply operation will either
	// reproduce them or fix them and we don't want fixed diagnostics to be
	// reported and we don't want duplicates either.
	var filteredDiags tfdiags.Diagnostics
	for _, diag := range run.Diagnostics {
		if rule, ok := addrs.DiagnosticOriginatesFromCheckRule(diag); ok && rule.Container.CheckableKind() == addrs.CheckableCheck {
			continue
		}
		filteredDiags = filteredDiags.Append(diag)
	}
	run.Diagnostics = filteredDiags

	applyCtx, updated, applyDiags := runner.apply(plan, state, config, run, file, moduletest.Running, start)

	// Remove expected diagnostics, and add diagnostics in case anything that should have failed didn't.
	applyDiags = run.ValidateExpectedFailures(applyDiags)

	run.Diagnostics = run.Diagnostics.Append(applyDiags)
	if applyDiags.HasErrors() {
		run.Status = moduletest.Error
		// Even though the apply operation failed, the graph may have done
		// partial updates and the returned state should reflect this.
		return updated, true
	}

	resetVariables := runner.AddVariablesToConfig(config, variables)
	defer resetVariables()

	run.Diagnostics = run.Diagnostics.Append(variableDiags)
	if variableDiags.HasErrors() {
		run.Status = moduletest.Error
		return updated, true
	}

	if runner.Suite.Verbose {
		schemas, diags := planCtx.Schemas(config, plan.PlannedState)

		// If we're going to fail to render the plan, let's not fail the overall
		// test. It can still have succeeded. So we'll add the diagnostics, but
		// still report the test status as a success.
		if diags.HasErrors() {
			// This is very unlikely.
			diags = diags.Append(tfdiags.Sourceless(
				tfdiags.Warning,
				"Failed to print verbose output",
				fmt.Sprintf("Terraform failed to print the verbose output for %s, other diagnostics will contain more details as to why.", path.Join(file.Name, run.Name))))
		} else {
			run.Verbose = &moduletest.Verbose{
				Plan:         plan,
				State:        updated,
				Config:       config,
				Providers:    schemas.Providers,
				Provisioners: schemas.Provisioners,
			}
		}

		run.Diagnostics = run.Diagnostics.Append(diags)
	}

	// First, make the test context we can use to validate the assertions
	// of the
	ctx := applyCtx.TestContext(run, config, updated, plan, variables)

	// Second, evaluate the run block directly. We also pass in all the
	// previous contexts so this run block can refer to outputs from
	// previous run blocks.
	ctx.Evaluate(runner.PriorStates)

	// Now we've successfully validated this run block, lets add it into
	// our prior states so future run blocks can access it.
	runner.PriorStates[run.Name] = ctx

	return updated, true
}

func (runner *TestFileRunner) validate(config *configs.Config, run *moduletest.Run, file *moduletest.File, start int64) tfdiags.Diagnostics {
	log.Printf("[TRACE] TestFileRunner: called validate for %s/%s", file.Name, run.Name)

	var diags tfdiags.Diagnostics

	tfCtx, ctxDiags := terraform.NewContext(runner.Suite.Opts)
	diags = diags.Append(ctxDiags)
	if ctxDiags.HasErrors() {
		return diags
	}

	runningCtx, done := context.WithCancel(context.Background())

	var validateDiags tfdiags.Diagnostics
	go func() {
		defer logging.PanicHandler()
		defer done()

		log.Printf("[DEBUG] TestFileRunner: starting validate for %s/%s", file.Name, run.Name)
		validateDiags = tfCtx.Validate(config)
		log.Printf("[DEBUG] TestFileRunner: completed validate for  %s/%s", file.Name, run.Name)
	}()
	waitDiags, cancelled := runner.wait(tfCtx, runningCtx, run, file, nil, moduletest.Running, start)

	if cancelled {
		diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Test interrupted", "The test operation could not be completed due to an interrupt signal. Please read the remaining diagnostics carefully for any sign of failed state cleanup or dangling resources."))
	}

	diags = diags.Append(waitDiags)
	diags = diags.Append(validateDiags)

	return diags
}

func (runner *TestFileRunner) destroy(config *configs.Config, state *states.State, run *moduletest.Run, file *moduletest.File) (*states.State, tfdiags.Diagnostics) {
	log.Printf("[TRACE] TestFileRunner: called destroy for %s/%s", file.Name, run.Name)

	if state.Empty() {
		// Nothing to do!
		return state, nil
	}

	var diags tfdiags.Diagnostics

	variables, variableDiags := runner.GetVariables(config, run, nil)
	diags = diags.Append(variableDiags)

	if diags.HasErrors() {
		return state, diags
	}

	// During the destroy operation, we don't add warnings from this operation.
	// Anything that would have been reported here was already reported during
	// the original plan, and a successful destroy operation is the only thing
	// we care about.
	setVariables, _ := runner.FilterVariablesToConfig(config, variables)

	planOpts := &terraform.PlanOpts{
		Mode:         plans.DestroyMode,
		SetVariables: setVariables,
		Overrides:    mocking.PackageOverrides(run.Config, file.Config, config),
	}

	tfCtx, ctxDiags := terraform.NewContext(runner.Suite.Opts)
	diags = diags.Append(ctxDiags)
	if ctxDiags.HasErrors() {
		return state, diags
	}

	runningCtx, done := context.WithCancel(context.Background())

	start := time.Now().UTC().UnixMilli()
	runner.Suite.View.Run(run, file, moduletest.TearDown, 0)

	var plan *plans.Plan
	var planDiags tfdiags.Diagnostics
	go func() {
		defer logging.PanicHandler()
		defer done()

		log.Printf("[DEBUG] TestFileRunner: starting destroy plan for %s/%s", file.Name, run.Name)
		plan, planDiags = tfCtx.Plan(config, state, planOpts)
		log.Printf("[DEBUG] TestFileRunner: completed destroy plan for %s/%s", file.Name, run.Name)
	}()
	waitDiags, cancelled := runner.wait(tfCtx, runningCtx, run, file, nil, moduletest.TearDown, start)

	if cancelled {
		diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Test interrupted", "The test operation could not be completed due to an interrupt signal. Please read the remaining diagnostics carefully for any sign of failed state cleanup or dangling resources."))
	}

	diags = diags.Append(waitDiags)
	diags = diags.Append(planDiags)

	if diags.HasErrors() {
		return state, diags
	}

	_, updated, applyDiags := runner.apply(plan, state, config, run, file, moduletest.TearDown, start)
	diags = diags.Append(applyDiags)
	return updated, diags
}

func (runner *TestFileRunner) plan(config *configs.Config, state *states.State, run *moduletest.Run, file *moduletest.File, variables terraform.InputValues, references []*addrs.Reference, start int64) (*terraform.Context, *plans.Plan, tfdiags.Diagnostics) {
	log.Printf("[TRACE] TestFileRunner: called plan for %s/%s", file.Name, run.Name)

	var diags tfdiags.Diagnostics

	targets, targetDiags := run.GetTargets()
	diags = diags.Append(targetDiags)

	replaces, replaceDiags := run.GetReplaces()
	diags = diags.Append(replaceDiags)

	if diags.HasErrors() {
		return nil, nil, diags
	}

	planOpts := &terraform.PlanOpts{
		Mode: func() plans.Mode {
			switch run.Config.Options.Mode {
			case configs.RefreshOnlyTestMode:
				return plans.RefreshOnlyMode
			default:
				return plans.NormalMode
			}
		}(),
		Targets:            targets,
		ForceReplace:       replaces,
		SkipRefresh:        !run.Config.Options.Refresh,
		SetVariables:       variables,
		ExternalReferences: references,
		Overrides:          mocking.PackageOverrides(run.Config, file.Config, config),
	}

	tfCtx, ctxDiags := terraform.NewContext(runner.Suite.Opts)
	diags = diags.Append(ctxDiags)
	if ctxDiags.HasErrors() {
		return nil, nil, diags
	}

	runningCtx, done := context.WithCancel(context.Background())

	var plan *plans.Plan
	var planDiags tfdiags.Diagnostics
	go func() {
		defer logging.PanicHandler()
		defer done()

		log.Printf("[DEBUG] TestFileRunner: starting plan for %s/%s", file.Name, run.Name)
		plan, planDiags = tfCtx.Plan(config, state, planOpts)
		log.Printf("[DEBUG] TestFileRunner: completed plan for %s/%s", file.Name, run.Name)
	}()
	waitDiags, cancelled := runner.wait(tfCtx, runningCtx, run, file, nil, moduletest.Running, start)

	if cancelled {
		diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Test interrupted", "The test operation could not be completed due to an interrupt signal. Please read the remaining diagnostics carefully for any sign of failed state cleanup or dangling resources."))
	}

	diags = diags.Append(waitDiags)
	diags = diags.Append(planDiags)

	return tfCtx, plan, diags
}

func (runner *TestFileRunner) apply(plan *plans.Plan, state *states.State, config *configs.Config, run *moduletest.Run, file *moduletest.File, progress moduletest.Progress, start int64) (*terraform.Context, *states.State, tfdiags.Diagnostics) {
	log.Printf("[TRACE] TestFileRunner: called apply for %s/%s", file.Name, run.Name)

	var diags tfdiags.Diagnostics

	// If things get cancelled while we are executing the apply operation below
	// we want to print out all the objects that we were creating so the user
	// can verify we managed to tidy everything up possibly.
	//
	// Unfortunately, this creates a race condition as the apply operation can
	// edit the plan (by removing changes once they are applied) while at the
	// same time our cancellation process will try to read the plan.
	//
	// We take a quick copy of the changes we care about here, which will then
	// be used in place of the plan when we print out the objects to be created
	// as part of the cancellation process.
	var created []*plans.ResourceInstanceChangeSrc
	for _, change := range plan.Changes.Resources {
		if change.Action != plans.Create {
			continue
		}
		created = append(created, change)
	}

	tfCtx, ctxDiags := terraform.NewContext(runner.Suite.Opts)
	diags = diags.Append(ctxDiags)
	if ctxDiags.HasErrors() {
		return nil, state, diags
	}

	runningCtx, done := context.WithCancel(context.Background())

	var updated *states.State
	var applyDiags tfdiags.Diagnostics

	go func() {
		defer logging.PanicHandler()
		defer done()
		log.Printf("[DEBUG] TestFileRunner: starting apply for %s/%s", file.Name, run.Name)
		updated, applyDiags = tfCtx.Apply(plan, config, nil)
		log.Printf("[DEBUG] TestFileRunner: completed apply for %s/%s", file.Name, run.Name)
	}()
	waitDiags, cancelled := runner.wait(tfCtx, runningCtx, run, file, created, progress, start)

	if cancelled {
		diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Test interrupted", "The test operation could not be completed due to an interrupt signal. Please read the remaining diagnostics carefully for any sign of failed state cleanup or dangling resources."))
	}

	diags = diags.Append(waitDiags)
	diags = diags.Append(applyDiags)

	return tfCtx, updated, diags
}

func (runner *TestFileRunner) wait(ctx *terraform.Context, runningCtx context.Context, run *moduletest.Run, file *moduletest.File, created []*plans.ResourceInstanceChangeSrc, progress moduletest.Progress, start int64) (diags tfdiags.Diagnostics, cancelled bool) {
	var identifier string
	if file == nil {
		identifier = "validate"
	} else {
		identifier = file.Name
		if run != nil {
			identifier = fmt.Sprintf("%s/%s", identifier, run.Name)
		}
	}
	log.Printf("[TRACE] TestFileRunner: waiting for execution during %s", identifier)

	// Keep track of when the execution is actually finished.
	finished := false

	// This function handles what happens when the user presses the second
	// interrupt. This is a "hard cancel", we are going to stop doing whatever
	// it is we're doing. This means even if we're halfway through creating or
	// destroying infrastructure we just give up.
	handleCancelled := func() {
		log.Printf("[DEBUG] TestFileRunner: test execution cancelled during %s", identifier)

		states := make(map[*moduletest.Run]*states.State)
		states[nil] = runner.RelevantStates[MainStateIdentifier].State
		for key, module := range runner.RelevantStates {
			if key == MainStateIdentifier {
				continue
			}
			states[module.Run] = module.State
		}
		runner.Suite.View.FatalInterruptSummary(run, file, states, created)

		cancelled = true
		go ctx.Stop()

		for !finished {
			select {
			case <-time.After(2 * time.Second):
				// Print an update while we're waiting.
				now := time.Now().UTC().UnixMilli()
				runner.Suite.View.Run(run, file, progress, now-start)
			case <-runningCtx.Done():
				// Just wait for things to finish now, the overall test execution will
				// exit early if this takes too long.
				finished = true
			}
		}

	}

	// This function handles what happens when the user presses the first
	// interrupt. This is essentially a "soft cancel", we're not going to do
	// anything but just wait for things to finish safely. But, we do listen
	// for the crucial second interrupt which will prompt a hard stop / cancel.
	handleStopped := func() {
		log.Printf("[DEBUG] TestFileRunner: test execution stopped during %s", identifier)

		for !finished {
			select {
			case <-time.After(2 * time.Second):
				// Print an update while we're waiting.
				now := time.Now().UTC().UnixMilli()
				runner.Suite.View.Run(run, file, progress, now-start)
			case <-runner.Suite.CancelledCtx.Done():
				// We've been asked again. This time we stop whatever we're doing
				// and abandon all attempts to do anything reasonable.
				handleCancelled()
			case <-runningCtx.Done():
				// Do nothing, we finished safely and skipping the remaining tests
				// will be handled elsewhere.
				finished = true
			}
		}

	}

	for !finished {
		select {
		case <-time.After(2 * time.Second):
			// Print an update while we're waiting.
			now := time.Now().UTC().UnixMilli()
			runner.Suite.View.Run(run, file, progress, now-start)
		case <-runner.Suite.StoppedCtx.Done():
			handleStopped()
		case <-runner.Suite.CancelledCtx.Done():
			handleCancelled()
		case <-runningCtx.Done():
			// The operation exited normally.
			finished = true
		}
	}

	return diags, cancelled
}

func (runner *TestFileRunner) cleanup(file *moduletest.File) {
	var diags tfdiags.Diagnostics

	log.Printf("[TRACE] TestStateManager: cleaning up state for %s", file.Name)

	if runner.Suite.Cancelled {
		// Don't try and clean anything up if the execution has been cancelled.
		log.Printf("[DEBUG] TestStateManager: skipping state cleanup for %s due to cancellation", file.Name)
		return
	}

	// First, we'll clean up the main state.
	main := runner.RelevantStates[MainStateIdentifier]

	updated := main.State
	if main.Run == nil {
		if !main.State.Empty() {
			log.Printf("[ERROR] TestFileRunner: found inconsistent run block and state file in %s", file.Name)
			diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Inconsistent state", fmt.Sprintf("Found inconsistent state while cleaning up %s. This is a bug in Terraform - please report it", file.Name)))
		}
	} else {
		reset, configDiags := configtest.TransformConfigForTest(runner.Suite.Config, main.Run, file, runner.globalVariables, runner.PriorStates, runner.Suite.configProviders[MainStateIdentifier])
		diags = diags.Append(configDiags)

		if !configDiags.HasErrors() {
			var destroyDiags tfdiags.Diagnostics
			updated, destroyDiags = runner.destroy(runner.Suite.Config, main.State, main.Run, file)
			diags = diags.Append(destroyDiags)
		}

		reset()
	}

	if !updated.Empty() {
		// Then we failed to adequately clean up the state, so mark success
		// as false.
		file.Status = moduletest.Error
	}
	runner.Suite.View.DestroySummary(diags, main.Run, file, updated)

	if runner.Suite.Cancelled {
		// In case things were cancelled during the last execution.
		return
	}

	var states []*TestFileState
	for key, state := range runner.RelevantStates {
		if key == MainStateIdentifier {
			// We processed the main state above.
			continue
		}

		if state.Run == nil {
			if state.State.Empty() {
				// We can see a run block being empty when the state is empty if
				// a module was only used to execute plan commands. So this is
				// okay, and means we have nothing to cleanup so we'll just
				// skip it.
				continue
			}
			log.Printf("[ERROR] TestFileRunner: found inconsistent run block and state file in %s for module %s", file.Name, key)

			// Otherwise something bad has happened, and we have no way to
			// recover from it. This shouldn't happen in reality, but we'll
			// print a diagnostic instead of panicking later.

			var diags tfdiags.Diagnostics
			diags = diags.Append(tfdiags.Sourceless(tfdiags.Error, "Inconsistent state", fmt.Sprintf("Found inconsistent state while cleaning up %s. This is a bug in Terraform - please report it", file.Name)))
			file.Status = moduletest.Error
			runner.Suite.View.DestroySummary(diags, nil, file, state.State)
			continue
		}

		states = append(states, state)
	}

	slices.SortFunc(states, func(a, b *TestFileState) int {
		// We want to clean up later run blocks first. So, we'll sort this in
		// reverse according to index. This means larger indices first.
		return b.Run.Index - a.Run.Index
	})

	// Then we'll clean up the additional states for custom modules in reverse
	// order.
	for _, state := range states {
		log.Printf("[DEBUG] TestStateManager: cleaning up state for %s/%s", file.Name, state.Run.Name)

		if runner.Suite.Cancelled {
			// In case the cancellation came while a previous state was being
			// destroyed.
			log.Printf("[DEBUG] TestStateManager: skipping state cleanup for %s/%s due to cancellation", file.Name, state.Run.Name)
			return
		}

		var diags tfdiags.Diagnostics

		reset, configDiags := configtest.TransformConfigForTest(state.Run.Config.ConfigUnderTest, state.Run, file, runner.globalVariables, runner.PriorStates, runner.Suite.configProviders[state.Run.Config.Module.Source.String()])
		diags = diags.Append(configDiags)

		updated := state.State
		if !diags.HasErrors() {
			var destroyDiags tfdiags.Diagnostics
			updated, destroyDiags = runner.destroy(state.Run.Config.ConfigUnderTest, state.State, state.Run, file)
			diags = diags.Append(destroyDiags)
		}

		if !updated.Empty() {
			// Then we failed to adequately clean up the state, so mark success
			// as false.
			file.Status = moduletest.Error
		}
		runner.Suite.View.DestroySummary(diags, state.Run, file, updated)

		reset()
	}
}

// GetVariables builds the terraform.InputValues required for the provided run
// block. It pulls the relevant variables (ie. the variables needed for the
// run block) from the total pool of all available variables, and converts them
// into input values.
//
// As a run block can reference variables defined within the file and are not
// actually defined within the configuration, this function actually returns
// more variables than are required by the config. FilterVariablesToConfig
// should be called before trying to use these variables within a Terraform
// plan, apply, or destroy operation.
func (runner *TestFileRunner) GetVariables(config *configs.Config, run *moduletest.Run, references []*addrs.Reference) (terraform.InputValues, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics

	// relevantVariables contains the variables that are of interest to this
	// run block. We can have variables defined at the global level and at the
	// file level that this run block doesn't need so we're going to make a
	// quick list of the variables that are actually relevant.
	relevantVariables := make(map[string]bool)

	// First, we'll check to see which variables the run block assertions
	// reference.
	for _, reference := range references {
		if addr, ok := reference.Subject.(addrs.InputVariable); ok {
			relevantVariables[addr.Name] = true
		}
	}

	// Second, we'll check to see which variables the run block variables
	// themselves reference. We might be processing variables just for the file
	// so the run block itself could be nil.
	for _, expr := range run.Config.Variables {
		for _, variable := range expr.Variables() {
			reference, referenceDiags := addrs.ParseRefFromTestingScope(variable)
			diags = diags.Append(referenceDiags)
			if reference != nil {
				if addr, ok := reference.Subject.(addrs.InputVariable); ok {
					relevantVariables[addr.Name] = true
				}
			}
		}
	}

	// Finally, we'll check to see which variables are actually defined within
	// the configuration.
	for name := range config.Module.Variables {
		relevantVariables[name] = true
	}

	// Now we know which variables are actually needed by this run block.

	// We're going to run over all the sets of variables we have access to:
	//   - Global variables, from the CLI / env vars / .tfvars files.
	//   - File variables, defined within the `variables` block in the file.
	//   - Run variables, defined within the `variables` block in this run.
	//   - ConfigVariables variables, defined directly within the config.
	values := make(terraform.InputValues)

	// First, let's look at the global variables.
	for name, value := range runner.globalVariables {
		if !relevantVariables[name] {
			// Then this run block doesn't need this value.
			continue
		}

		// By default, we parse global variables as HCL inputs.
		parsingMode := configs.VariableParseHCL

		cfg, exists := config.Module.Variables[name]
		if exists {
			// Unless we have some configuration that can actually tell us
			// what parsing mode to use.
			parsingMode = cfg.ParsingMode
		}

		var valueDiags tfdiags.Diagnostics
		values[name], valueDiags = value.ParseVariableValue(parsingMode)
		diags = diags.Append(valueDiags)
	}

	// We need to also process all global variables for tests.
	// If we run "terrafrom test" from the testing directory itself,
	// the filepath.Dir(file.Name) is equal to "." so we need to extend the logic
	if filepath.Dir(file.Name) == runner.Suite.TestingDirectory || filepath.Dir(file.Name) == "."{
		for name, value := range runner.Suite.GlobalTestVariables {
			if !relevantVariables[name] {
				// Then this run block doesn't need this value.
				continue
			}

			// By default, we parse global variables as HCL inputs.
			parsingMode := configs.VariableParseHCL

			cfg, exists := config.Module.Variables[name]
			if exists {
				// Unless we have some configuration that can actually tell us
				// what parsing mode to use.
				parsingMode = cfg.ParsingMode
			}

			var valueDiags tfdiags.Diagnostics
			values[name], valueDiags = value.ParseVariableValue(parsingMode)
			diags = diags.Append(valueDiags)
		}
	}

	// Second, we'll check the file level variables.
	for name, expr := range file.Config.Variables {
		if !relevantVariables[name] {
			continue
		}

		value, valueDiags := expr.Value(nil)
		diags = diags.Append(valueDiags)

		values[name] = &terraform.InputValue{
			Value:       value,
			SourceType:  terraform.ValueFromConfig,
			SourceRange: tfdiags.SourceRangeFromHCL(expr.Range()),
		}
	}

	// Third, we'll check the run level variables.

	// This is a bit more complicated, as the run level variables can reference
	// previously defined variables.

	ctx, ctxDiags := runner.ctx(run, file, values)
	diags = diags.Append(ctxDiags)

	var failedContext bool
	if ctxDiags.HasErrors() {
		// If we couldn't build the context, we won't actually process these
		// variables. Instead, we'll fill them with an empty value but still
		// make a note that the user did provide them.
		failedContext = true
	}

	for name, expr := range run.Config.Variables {
		if !relevantVariables[name] {
			// We'll add a warning for this. Since we're right in the run block
			// users shouldn't be defining variables that are not relevant.
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagWarning,
				Summary:  "Value for undeclared variable",
				Detail:   fmt.Sprintf("The module under test does not declare a variable named %q, but it is declared in run block %q.", name, run.Name),
				Subject:  expr.Range().Ptr(),
			})
			continue
		}

		value := cty.NilVal
		if !failedContext {
			var valueDiags hcl.Diagnostics
			value, valueDiags = expr.Value(ctx)
			diags = diags.Append(valueDiags)
		}

		values[name] = &terraform.InputValue{
			Value:       value,
			SourceType:  terraform.ValueFromConfig,
			SourceRange: tfdiags.SourceRangeFromHCL(expr.Range()),
		}
	}

	// Finally, we check the configuration again. This is where we'll discover
	// if there's any missing variables and fill in any optional variables that
	// don't have a value already.

	for name, variable := range config.Module.Variables {
		if _, exists := values[name]; exists {
			// Then we've provided a variable for this. It's all good.
			continue
		}

		// Otherwise, we're going to give these variables a value. They'll be
		// processed by the Terraform graph and provided a default value later
		// if they have one.

		if variable.Required() {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "No value for required variable",
				Detail: fmt.Sprintf("The module under test for run block %q has a required variable %q with no set value. Use a -var or -var-file command line argument or add this variable into a \"variables\" block within the test file or run block.",
					run.Name, variable.Name),
				Subject: variable.DeclRange.Ptr(),
			})

			values[name] = &terraform.InputValue{
				Value:       cty.DynamicVal,
				SourceType:  terraform.ValueFromConfig,
				SourceRange: tfdiags.SourceRangeFromHCL(variable.DeclRange),
			}
		} else {
			values[name] = &terraform.InputValue{
				Value:       cty.NilVal,
				SourceType:  terraform.ValueFromConfig,
				SourceRange: tfdiags.SourceRangeFromHCL(variable.DeclRange),
			}
		}

	}

	return values, diags
}

// FilterVariablesToConfig filters the provided values down into only the values
// required by the specified configuration.
//
// This function is essentially the opposite of AddVariablesToConfig which
// makes the config match the variables rather than the variables match the
// config.
//
// This function can only return warnings, and the callers can rely on this so
// please check the callers of this function if you add any error diagnostics.
func (runner *TestFileRunner) FilterVariablesToConfig(config *configs.Config, values terraform.InputValues) (terraform.InputValues, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics

	filtered := make(terraform.InputValues)
	for name, value := range values {
		variableConfig, exists := config.Module.Variables[name]
		if !exists {
			// Only include values that are actually required by the config.
			continue
		}

		if marks.Has(value.Value, marks.Sensitive) {
			unmarkedValue, _ := value.Value.Unmark()
			if !variableConfig.Sensitive {
				// Then we are passing a sensitive value into a non-sensitive
				// variable. Let's add a warning and tell the user they should
				// mark the config as sensitive as well. If the config variable
				// is sensitive, then we don't need to worry.
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagWarning,
					Summary:  "Sensitive metadata on variable lost",
					Detail:   fmt.Sprintf("The input variable is marked as sensitive, while the receiving configuration is not. The underlying sensitive information may be exposed when var.%s is referenced. Mark the variable block in the configuration as sensitive to resolve this warning.", variableConfig.Name),
					Subject:  value.SourceRange.ToHCL().Ptr(),
				})
			}

			// Set the unmarked value into the input value.
			value = &terraform.InputValue{
				Value:       unmarkedValue,
				SourceType:  value.SourceType,
				SourceRange: value.SourceRange,
			}
		}

		filtered[name] = value
	}
	return filtered, diags
}

// AddVariablesToConfig extends the provided config to ensure it has definitions
// for all specified variables.
//
// This function is essentially the opposite of FilterVariablesToConfig which
// makes the variables match the config rather than the config match the
// variables.
func (runner *TestFileRunner) AddVariablesToConfig(config *configs.Config, variables terraform.InputValues) func() {

	// If we have got variable values from the test file we need to make sure
	// they have an equivalent entry in the configuration. We're going to do
	// that dynamically here.

	// First, take a backup of the existing configuration so we can easily
	// restore it later.
	currentVars := make(map[string]*configs.Variable)
	for name, variable := range config.Module.Variables {
		currentVars[name] = variable
	}

	for name, value := range variables {
		if _, exists := config.Module.Variables[name]; exists {
			continue
		}

		config.Module.Variables[name] = &configs.Variable{
			Name:           name,
			Type:           value.Value.Type(),
			ConstraintType: value.Value.Type(),
			DeclRange:      value.SourceRange.ToHCL(),
		}
	}

	// We return a function that will reset the variables within the config so
	// it can be used again.
	return func() {
		config.Module.Variables = currentVars
	}
}

// initVariables initialises the globalVariables within the test runner by
// merging the global variables from the test suite into the variables from
// the file.
func (runner *TestFileRunner) initVariables(file *moduletest.File) {
	runner.globalVariables = make(map[string]backend.UnparsedVariableValue)
	for name, value := range runner.Suite.GlobalVariables {
		runner.globalVariables[name] = value
	}
	for name, expr := range file.Config.Variables {
		runner.globalVariables[name] = unparsedTestVariableValue{expr}
	}
}

func (runner *TestFileRunner) gatherProviders(key string, config *configs.Config) {
	if _, exists := runner.Suite.configProviders[key]; exists {
		// Then we've processed this key before, so skip it.
		return
	}

	providers := make(map[string]bool)

	// First, let's look at the required providers first.
	for _, provider := range config.Module.ProviderRequirements.RequiredProviders {
		providers[provider.Name] = true
		for _, alias := range provider.Aliases {
			providers[alias.StringCompact()] = true
		}
	}

	// Second, we look at the defined provider configs.
	for _, provider := range config.Module.ProviderConfigs {
		providers[provider.Addr().StringCompact()] = true
	}

	// Third, we look at the resources and data sources.
	for _, resource := range config.Module.ManagedResources {
		if resource.ProviderConfigRef != nil {
			providers[resource.ProviderConfigRef.String()] = true
			continue
		}
		providers[resource.Provider.Type] = true
	}
	for _, datasource := range config.Module.DataResources {
		if datasource.ProviderConfigRef != nil {
			providers[datasource.ProviderConfigRef.String()] = true
			continue
		}
		providers[datasource.Provider.Type] = true
	}

	// Finally, we look at any module calls to see if any providers are used
	// in there.
	for _, module := range config.Module.ModuleCalls {
		for _, provider := range module.Providers {
			providers[provider.InParent.String()] = true
		}
	}

	runner.Suite.configProviders[key] = providers
}
