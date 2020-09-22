package azure

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform/backend"
	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/states/remote"
	"github.com/tombuildsstuff/giovanni/storage/2018-11-09/blob/blobs"
)

func TestRemoteClient_impl(t *testing.T) {
	var _ remote.Client = new(RemoteClient)
	var _ remote.ClientLocker = new(RemoteClient)
}

func TestRemoteClientAccessKeyBasic(t *testing.T) {
	testAccAzureBackend(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	b := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"access_key":           res.storageAccountAccessKey,
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	state, err := b.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestClient(t, state.(*remote.State).Client)
}

func TestRemoteClientManagedServiceIdentityBasic(t *testing.T) {
	testAccAzureBackendRunningInAzure(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	b := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"resource_group_name":  res.resourceGroup,
		"use_msi":              true,
		"subscription_id":      os.Getenv("ARM_SUBSCRIPTION_ID"),
		"tenant_id":            os.Getenv("ARM_TENANT_ID"),
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	state, err := b.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestClient(t, state.(*remote.State).Client)
}

func TestRemoteClientSasTokenBasic(t *testing.T) {
	testAccAzureBackend(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	sasToken, err := buildSasToken(res.storageAccountName, res.storageAccountAccessKey)
	if err != nil {
		t.Fatalf("Error building SAS Token: %+v", err)
	}

	b := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"sas_token":            *sasToken,
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	state, err := b.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestClient(t, state.(*remote.State).Client)
}

func TestRemoteClientServicePrincipalBasic(t *testing.T) {
	testAccAzureBackend(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	b := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"resource_group_name":  res.resourceGroup,
		"subscription_id":      os.Getenv("ARM_SUBSCRIPTION_ID"),
		"tenant_id":            os.Getenv("ARM_TENANT_ID"),
		"client_id":            os.Getenv("ARM_CLIENT_ID"),
		"client_secret":        os.Getenv("ARM_CLIENT_SECRET"),
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	state, err := b.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestClient(t, state.(*remote.State).Client)
}

func TestRemoteClientAccessKeyLocks(t *testing.T) {
	testAccAzureBackend(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	b1 := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"access_key":           res.storageAccountAccessKey,
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	b2 := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"access_key":           res.storageAccountAccessKey,
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	s1, err := b1.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	s2, err := b2.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestRemoteLocks(t, s1.(*remote.State).Client, s2.(*remote.State).Client)
}

func TestRemoteClientServicePrincipalLocks(t *testing.T) {
	testAccAzureBackend(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	b1 := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"resource_group_name":  res.resourceGroup,
		"subscription_id":      os.Getenv("ARM_SUBSCRIPTION_ID"),
		"tenant_id":            os.Getenv("ARM_TENANT_ID"),
		"client_id":            os.Getenv("ARM_CLIENT_ID"),
		"client_secret":        os.Getenv("ARM_CLIENT_SECRET"),
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	b2 := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name": res.storageAccountName,
		"container_name":       res.storageContainerName,
		"key":                  res.storageKeyName,
		"resource_group_name":  res.resourceGroup,
		"subscription_id":      os.Getenv("ARM_SUBSCRIPTION_ID"),
		"tenant_id":            os.Getenv("ARM_TENANT_ID"),
		"client_id":            os.Getenv("ARM_CLIENT_ID"),
		"client_secret":        os.Getenv("ARM_CLIENT_SECRET"),
		"environment":          os.Getenv("ARM_ENVIRONMENT"),
		"endpoint":             os.Getenv("ARM_ENDPOINT"),
	})).(*Backend)

	s1, err := b1.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	s2, err := b2.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestRemoteLocks(t, s1.(*remote.State).Client, s2.(*remote.State).Client)
}

func TestPutMaintainsMetaData(t *testing.T) {
	testAccAzureBackend(t)
	rs := acctest.RandString(4)
	res := testResourceNames(rs, "testState")
	armClient := buildTestClient(t, res)

	ctx := context.TODO()
	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	headerName := "acceptancetest"
	expectedValue := "f3b56bad-33ad-4b93-a600-7a66e9cbd1eb"

	client, err := armClient.getBlobClient(ctx)
	if err != nil {
		t.Fatalf("Error building Blob Client: %+v", err)
	}

	_, err = client.PutBlockBlob(ctx, res.storageAccountName, res.storageContainerName, res.storageKeyName, blobs.PutBlockBlobInput{})
	if err != nil {
		t.Fatalf("Error Creating Block Blob: %+v", err)
	}

	blobReference, err := client.GetProperties(ctx, res.storageAccountName, res.storageContainerName, res.storageKeyName, blobs.GetPropertiesInput{})
	if err != nil {
		t.Fatalf("Error loading MetaData: %+v", err)
	}

	blobReference.MetaData[headerName] = expectedValue
	opts := blobs.SetMetaDataInput{
		MetaData: blobReference.MetaData,
	}
	_, err = client.SetMetaData(ctx, res.storageAccountName, res.storageContainerName, res.storageKeyName, opts)
	if err != nil {
		t.Fatalf("Error setting MetaData: %+v", err)
	}

	// update the metadata using the Backend
	remoteClient := RemoteClient{
		keyName:       res.storageKeyName,
		containerName: res.storageContainerName,
		accountName:   res.storageAccountName,

		giovanniBlobClient: *client,
	}

	bytes := []byte(acctest.RandString(20))
	err = remoteClient.Put(bytes)
	if err != nil {
		t.Fatalf("Error putting data: %+v", err)
	}

	// Verify it still exists
	blobReference, err = client.GetProperties(ctx, res.storageAccountName, res.storageContainerName, res.storageKeyName, blobs.GetPropertiesInput{})
	if err != nil {
		t.Fatalf("Error loading MetaData: %+v", err)
	}

	if blobReference.MetaData[headerName] != expectedValue {
		t.Fatalf("%q was not set to %q in the MetaData: %+v", headerName, expectedValue, blobReference.MetaData)
	}
}

func TestRemoteClientWithKeyVaultEncryption(t *testing.T) {
	testAccAzureBackend(t)

	rs := acctest.RandString(4)

	keyVaultName := fmt.Sprintf("keyvaultterraform%s", rs)
	keyName := "myKey"
	keyIdentifier := fmt.Sprintf("https://%s.vault.azure.net/keys/%s", keyVaultName, keyName)

	ctx := context.TODO()
	res := testResourceNamesWithKeyVault(rs, "testState", keyVaultName, keyName)
	armClient := buildTestClientWithKeyVault(t, res)

	err := armClient.buildTestResources(ctx, &res)
	defer armClient.destroyTestResources(ctx, res)
	if err != nil {
		t.Fatalf("Error creating Test Resources: %q", err)
	}

	b := backend.TestBackendConfig(t, New(), backend.TestWrapConfig(map[string]interface{}{
		"storage_account_name":     res.storageAccountName,
		"container_name":           res.storageContainerName,
		"key":                      res.storageKeyName,
		"resource_group_name":      res.resourceGroup,
		"subscription_id":          os.Getenv("ARM_SUBSCRIPTION_ID"),
		"tenant_id":                os.Getenv("ARM_TENANT_ID"),
		"client_id":                os.Getenv("ARM_CLIENT_ID"),
		"client_secret":            os.Getenv("ARM_CLIENT_SECRET"),
		"environment":              os.Getenv("ARM_ENVIRONMENT"),
		"key_vault_key_identifier": keyIdentifier,
	})).(*Backend)

	state, err := b.StateMgr(backend.DefaultStateName)
	if err != nil {
		t.Fatal(err)
	}

	remote.TestClient(t, state.(*remote.State).Client)
}
