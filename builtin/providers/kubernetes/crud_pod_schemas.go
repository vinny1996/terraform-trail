package kubernetes

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/util"
)

func createStringList(_values []interface{}) []string {
	values := make([]string, len(_values))
	for i, v := range _values {
		values[i] = v.(string)
	}
	return values
}

func createVolumes(_volumes []interface{}) []api.Volume {
	volumes := make([]api.Volume, len(_volumes))
	for i, v := range _volumes {
		_volume := v.(map[string]interface{})
		volume := api.Volume{}

		volume.Name = _volume["name"].(string)

		volumeSource := createVolumeSource(_volume["volume_source"].([]interface{}))

		if volumeSource != nil {
			volume.HostPath = volumeSource.HostPath
			volume.EmptyDir = volumeSource.EmptyDir
			volume.GCEPersistentDisk = volumeSource.GCEPersistentDisk
			volume.AWSElasticBlockStore = volumeSource.AWSElasticBlockStore
			volume.GitRepo = volumeSource.GitRepo
			volume.Secret = volumeSource.Secret
			volume.NFS = volumeSource.NFS
			volume.ISCSI = volumeSource.ISCSI
			volume.Glusterfs = volumeSource.Glusterfs
			volume.PersistentVolumeClaim = volumeSource.PersistentVolumeClaim
			volume.Cinder = volumeSource.Cinder
			volume.CephFS = volumeSource.CephFS
			volume.Flocker = volumeSource.Flocker
			volume.DownwardAPI = volumeSource.DownwardAPI
			volume.FC = volumeSource.FC
		}

		volumes[i] = volume
	}

	return volumes
}

func createVolumeSource(_volume_sources []interface{}) *api.VolumeSource {
	if len(_volume_sources) == 0 {
		return nil
	} else {
		_volume_source := _volume_sources[0].(map[string]interface{})
		volumeSource := &api.VolumeSource{}

		if val, ok := _volume_source["host_path"]; ok {
			volumeSource.HostPath = createHostPathVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["empty_dir"]; ok {
			volumeSource.EmptyDir = createEmptyDirVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["gce_persistent_disk"]; ok {
			volumeSource.GCEPersistentDisk = createGcePersistentDiskVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["aws_elastic_block_store"]; ok {
			volumeSource.AWSElasticBlockStore = createAwsElasticBlockStoreVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["git_repo"]; ok {
			volumeSource.GitRepo = createGitRepoVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["sercret"]; ok {
			volumeSource.Secret = createSecretVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["nfs"]; ok {
			volumeSource.NFS = createNfsVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["iscsi"]; ok {
			volumeSource.ISCSI = createIscsiVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["gluster_fs"]; ok {
			volumeSource.Glusterfs = createGlusterfsVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["persistent_volume_claim"]; ok {
			volumeSource.PersistentVolumeClaim = createPersistentVolumeClaimVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["cinder"]; ok {
			volumeSource.Cinder = createCinderVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["cephfs"]; ok {
			volumeSource.CephFS = createCephFsVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["flocker"]; ok {
			volumeSource.Flocker = createFlockerVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["downward_api"]; ok {
			volumeSource.DownwardAPI = createDownwardApiVolumeSource(val.([]interface{}))
		}

		if val, ok := _volume_source["fc"]; ok {
			volumeSource.FC = createFcVolumeSource(val.([]interface{}))
		}

		return volumeSource
	}
}

func createLocalObjectReference(_local_object_references []interface{}) *api.LocalObjectReference {
	if len(_local_object_references) == 0 {
		return nil
	} else {
		_local_object_reference := _local_object_references[0].(map[string]interface{})
		localObjectReference := &api.LocalObjectReference{}

		if val, ok := _local_object_reference["name"]; ok {
			localObjectReference.Name = val.(string)
		}

		return localObjectReference
	}
}

func createHostPathVolumeSource(_host_paths []interface{}) *api.HostPathVolumeSource {
	if len(_host_paths) == 0 {
		return nil
	} else {
		_host_path := _host_paths[0].(map[string]interface{})
		hostPath := &api.HostPathVolumeSource{}

		if val, ok := _host_path["path"]; ok {
			hostPath.Path = val.(string)
		}

		return hostPath
	}
}

func createEmptyDirVolumeSource(_empty_dirs []interface{}) *api.EmptyDirVolumeSource {
	if len(_empty_dirs) == 0 {
		return nil
	} else {
		_empty_dir := _empty_dirs[0].(map[string]interface{})
		emptyDir := &api.EmptyDirVolumeSource{}

		if val, ok := _empty_dir["medium"]; ok {
			emptyDir.Medium = api.StorageMedium(val.(string))
		}

		return emptyDir
	}
}

func createGcePersistentDiskVolumeSource(_gce_persistent_disks []interface{}) *api.GCEPersistentDiskVolumeSource {
	if len(_gce_persistent_disks) == 0 {
		return nil
	} else {
		_gce_persistent_disk := _gce_persistent_disks[0].(map[string]interface{})
		gcePersistentDisk := &api.GCEPersistentDiskVolumeSource{}

		if val, ok := _gce_persistent_disk["pd_name"]; ok {
			gcePersistentDisk.PDName = val.(string)
		}

		if val, ok := _gce_persistent_disk["fs_type"]; ok {
			gcePersistentDisk.FSType = val.(string)
		}

		if val, ok := _gce_persistent_disk["partition"]; ok {
			gcePersistentDisk.Partition = val.(int)
		}

		if val, ok := _gce_persistent_disk["read_only"]; ok {
			gcePersistentDisk.ReadOnly = val.(bool)
		}

		return gcePersistentDisk
	}
}

func createAwsElasticBlockStoreVolumeSource(_aws_elastic_block_stores []interface{}) *api.AWSElasticBlockStoreVolumeSource {
	if len(_aws_elastic_block_stores) == 0 {
		return nil
	} else {
		_aws_elastic_block_store := _aws_elastic_block_stores[0].(map[string]interface{})
		awsElasticBlockStore := &api.AWSElasticBlockStoreVolumeSource{}

		if val, ok := _aws_elastic_block_store["volume_id"]; ok {
			awsElasticBlockStore.VolumeID = val.(string)
		}

		if val, ok := _aws_elastic_block_store["fs_type"]; ok {
			awsElasticBlockStore.FSType = val.(string)
		}

		if val, ok := _aws_elastic_block_store["partition"]; ok {
			awsElasticBlockStore.Partition = val.(int)
		}

		if val, ok := _aws_elastic_block_store["read_only"]; ok {
			awsElasticBlockStore.ReadOnly = val.(bool)
		}

		return awsElasticBlockStore
	}
}

func createGitRepoVolumeSource(_git_repos []interface{}) *api.GitRepoVolumeSource {
	if len(_git_repos) == 0 {
		return nil
	} else {
		_git_repo := _git_repos[0].(map[string]interface{})
		gitRepo := &api.GitRepoVolumeSource{}

		if val, ok := _git_repo["repository"]; ok {
			gitRepo.Repository = val.(string)
		}

		if val, ok := _git_repo["revision"]; ok {
			gitRepo.Revision = val.(string)
		}

		return gitRepo
	}
}

func createSecretVolumeSource(_secrets []interface{}) *api.SecretVolumeSource {
	if len(_secrets) == 0 {
		return nil
	} else {
		_secret := _secrets[0].(map[string]interface{})
		secret := &api.SecretVolumeSource{}

		if val, ok := _secret["secret_name"]; ok {
			secret.SecretName = val.(string)
		}

		return secret
	}
}

func createNfsVolumeSource(_nfss []interface{}) *api.NFSVolumeSource {
	if len(_nfss) == 0 {
		return nil
	} else {
		_nfs := _nfss[0].(map[string]interface{})
		nfs := &api.NFSVolumeSource{}

		if val, ok := _nfs["server"]; ok {
			nfs.Server = val.(string)
		}

		if val, ok := _nfs["path"]; ok {
			nfs.Path = val.(string)
		}

		if val, ok := _nfs["read_only"]; ok {
			nfs.ReadOnly = val.(bool)
		}

		return nfs
	}
}

func createIscsiVolumeSource(_iscsis []interface{}) *api.ISCSIVolumeSource {
	if len(_iscsis) == 0 {
		return nil
	} else {
		_iscsi := _iscsis[0].(map[string]interface{})
		iscsi := &api.ISCSIVolumeSource{}

		if val, ok := _iscsi["target_portal"]; ok {
			iscsi.TargetPortal = val.(string)
		}

		if val, ok := _iscsi["iqn"]; ok {
			iscsi.IQN = val.(string)
		}

		if val, ok := _iscsi["lun"]; ok {
			iscsi.Lun = val.(int)
		}

		if val, ok := _iscsi["fs_type"]; ok {
			iscsi.FSType = val.(string)
		}

		if val, ok := _iscsi["read_only"]; ok {
			iscsi.ReadOnly = val.(bool)
		}

		return iscsi
	}
}

func createGlusterfsVolumeSource(_glusterfss []interface{}) *api.GlusterfsVolumeSource {
	if len(_glusterfss) == 0 {
		return nil
	} else {
		_glusterfs := _glusterfss[0].(map[string]interface{})
		glusterfs := &api.GlusterfsVolumeSource{}

		if val, ok := _glusterfs["endpoints_name"]; ok {
			glusterfs.EndpointsName = val.(string)
		}

		if val, ok := _glusterfs["path"]; ok {
			glusterfs.Path = val.(string)
		}

		if val, ok := _glusterfs["read_only"]; ok {
			glusterfs.ReadOnly = val.(bool)
		}

		return glusterfs
	}
}

func createPersistentVolumeClaimVolumeSource(_persistent_volume_claims []interface{}) *api.PersistentVolumeClaimVolumeSource {
	if len(_persistent_volume_claims) == 0 {
		return nil
	} else {
		_persistent_volume_claim := _persistent_volume_claims[0].(map[string]interface{})
		persistentVolumeClaim := &api.PersistentVolumeClaimVolumeSource{}

		if val, ok := _persistent_volume_claim["claim_name"]; ok {
			persistentVolumeClaim.ClaimName = val.(string)
		}

		if val, ok := _persistent_volume_claim["read_only"]; ok {
			persistentVolumeClaim.ReadOnly = val.(bool)
		}

		return persistentVolumeClaim
	}
}

func createRbdVolumeSource(_rbds []interface{}) *api.RBDVolumeSource {
	if len(_rbds) == 0 {
		return nil
	} else {
		_rbd := _rbds[0].(map[string]interface{})
		rbd := &api.RBDVolumeSource{}

		if val, ok := _rbd["ceph_monitors"]; ok {
			rbd.CephMonitors = createStringList(val.([]interface{}))
		}

		if val, ok := _rbd["rbd_image"]; ok {
			rbd.RBDImage = val.(string)
		}

		if val, ok := _rbd["fs_type"]; ok {
			rbd.FSType = val.(string)
		}

		if val, ok := _rbd["rbd_pool"]; ok {
			rbd.RBDPool = val.(string)
		}

		if val, ok := _rbd["rados_user"]; ok {
			rbd.RadosUser = val.(string)
		}

		if val, ok := _rbd["keyring"]; ok {
			rbd.Keyring = val.(string)
		}

		if val, ok := _rbd["secret_ref"]; ok {
			rbd.SecretRef = createLocalObjectReference(val.([]interface{}))
		}

		if val, ok := _rbd["read_only"]; ok {
			rbd.ReadOnly = val.(bool)
		}

		return rbd
	}
}

func createCinderVolumeSource(_cinders []interface{}) *api.CinderVolumeSource {
	if len(_cinders) == 0 {
		return nil
	} else {
		_cinder := _cinders[0].(map[string]interface{})
		cinder := &api.CinderVolumeSource{}

		if val, ok := _cinder["volume_id"]; ok {
			cinder.VolumeID = val.(string)
		}

		if val, ok := _cinder["fs_type"]; ok {
			cinder.FSType = val.(string)
		}

		if val, ok := _cinder["read_only"]; ok {
			cinder.ReadOnly = val.(bool)
		}

		return cinder
	}
}

func createCephFsVolumeSource(_ceph_fss []interface{}) *api.CephFSVolumeSource {
	if len(_ceph_fss) == 0 {
		return nil
	} else {
		_ceph_fs := _ceph_fss[0].(map[string]interface{})
		cephFs := &api.CephFSVolumeSource{}

		if val, ok := _ceph_fs["monitors"]; ok {
			cephFs.Monitors = createStringList(val.([]interface{}))
		}

		if val, ok := _ceph_fs["user"]; ok {
			cephFs.User = val.(string)
		}

		if val, ok := _ceph_fs["secret_file"]; ok {
			cephFs.SecretFile = val.(string)
		}

		if val, ok := _ceph_fs["secret_ref"]; ok {
			cephFs.SecretRef = createLocalObjectReference(val.([]interface{}))
		}

		if val, ok := _ceph_fs["read_only"]; ok {
			cephFs.ReadOnly = val.(bool)
		}

		return cephFs
	}
}

func createFlockerVolumeSource(_flockers []interface{}) *api.FlockerVolumeSource {
	if len(_flockers) == 0 {
		return nil
	} else {
		_flocker := _flockers[0].(map[string]interface{})
		flocker := &api.FlockerVolumeSource{}

		if val, ok := _flocker["dataset_name"]; ok {
			flocker.DatasetName = val.(string)
		}

		return flocker
	}
}

func createDownwardApiVolumeSource(_downward_apis []interface{}) *api.DownwardAPIVolumeSource {
	if len(_downward_apis) == 0 {
		return nil
	} else {
		_downward_api := _downward_apis[0].(map[string]interface{})
		downwardApi := &api.DownwardAPIVolumeSource{}

		if val, ok := _downward_api["items"]; ok {
			downwardApi.Items = createDownwardApiVolumeFiles(val.([]interface{}))
		}

		return downwardApi
	}
}

func createDownwardApiVolumeFiles(_volume_files []interface{}) []api.DownwardAPIVolumeFile {
	volumeFiles := make([]api.DownwardAPIVolumeFile, len(_volume_files))
	for i, v := range _volume_files {
		volumeFile := api.DownwardAPIVolumeFile{}
		_volume_file := v.(map[string]interface{})

		volumeFile.Path = _volume_file["path"].(string)

		fieldRef := createObjectFieldSelector(_volume_file["field_ref"].([]interface{}))
		if fieldRef != nil {
			volumeFile.FieldRef = *fieldRef
		}

		volumeFiles[i] = volumeFile
	}

	return volumeFiles
}

func createFcVolumeSource(_fcs []interface{}) *api.FCVolumeSource {
	if len(_fcs) == 0 {
		return nil
	} else {
		_fc := _fcs[0].(map[string]interface{})
		fc := &api.FCVolumeSource{}

		if val, ok := _fc["target_wwns"]; ok {
			fc.TargetWWNs = createStringList(val.([]interface{}))
		}

		if val, ok := _fc["lun"]; ok {
			v := val.(int)
			fc.Lun = &v
		}

		if val, ok := _fc["fs_type"]; ok {
			fc.FSType = val.(string)
		}

		if val, ok := _fc["read_only"]; ok {
			fc.ReadOnly = val.(bool)
		}

		return fc
	}
}

func createContainers(_containers []interface{}) []api.Container {
	containers := make([]api.Container, len(_containers))
	for i, v := range _containers {
		_container := v.(map[string]interface{})
		container := &api.Container{}

		container.Name = _container["name"].(string)
		container.Image = _container["image"].(string)

		if val, ok := _container["command"]; ok {
			container.Command = createStringList(val.([]interface{}))
		}

		if val, ok := _container["args"]; ok {
			container.Args = createStringList(val.([]interface{}))
		}

		if val, ok := _container["working_dir"]; ok {
			container.WorkingDir = val.(string)
		}

		if val, ok := _container["container_port"]; ok {
			container.Ports = createContainerPorts(val.([]interface{}))
		}

		if val, ok := _container["env"]; ok {
			container.Env = createEnvVars(val.([]interface{}))
		}

		if val, ok := _container["resources"]; ok {
			resources := createResourceRequirements(val.([]interface{}))
			if resources != nil {
				container.Resources = *resources
			}
		}

		if val, ok := _container["volume_mount"]; ok {
			container.VolumeMounts = createVolumeMounts(val.([]interface{}))
		}

		if val, ok := _container["liveness_probe"]; ok {
			container.LivenessProbe = createProbe(val.([]interface{}))
		}

		if val, ok := _container["readiness_probe"]; ok {
			container.ReadinessProbe = createProbe(val.([]interface{}))
		}

		if val, ok := _container["lifecycle"]; ok {
			container.Lifecycle = createLifecycle(val.([]interface{}))
		}

		container.TerminationMessagePath = _container["termination_message_path"].(string)

		container.ImagePullPolicy = api.PullPolicy(_container["image_pull_policy"].(string))

		if val, ok := _container["security_context"]; ok {
			container.SecurityContext = createSecurityContext(val.([]interface{}))
		}

		if val, ok := _container["stdin"]; ok {
			container.Stdin = val.(bool)
		}

		if val, ok := _container["tty"]; ok {
			container.TTY = val.(bool)
		}

		containers[i] = *container
	}

	return containers
}

func createContainerPorts(_ports []interface{}) []api.ContainerPort {
	ports := make([]api.ContainerPort, len(_ports))
	for i, v := range _ports {
		_port := v.(map[string]interface{})
		port := api.ContainerPort{}

		if val, ok := _port["name"]; ok {
			port.Name = val.(string)
		}

		if val, ok := _port["host_port"]; ok {
			port.HostPort = val.(int)
		}

		port.ContainerPort = _port["container_port"].(int)

		port.Protocol = api.Protocol(_port["Protocol"].(string))

		if val, ok := _port["host_ip"]; ok {
			port.HostIP = val.(string)
		}

		ports[i] = port
	}

	return ports
}

func createEnvVars(_env_vars []interface{}) []api.EnvVar {
	env_vars := make([]api.EnvVar, len(_env_vars))
	for i, v := range _env_vars {
		_env_var := v.(map[string]interface{})
		env_var := api.EnvVar{}

		env_var.Name = _env_var["name"].(string)

		if val, ok := _env_var["value"]; ok {
			env_var.Value = val.(string)
		}

		if val, ok := _env_var["value_from"]; ok {
			env_var.ValueFrom = createEnvVarSource(val.([]interface{}))
		}

		env_vars[i] = env_var
	}

	return env_vars
}

func createEnvVarSource(_env_var_sources []interface{}) *api.EnvVarSource {
	if len(_env_var_sources) == 0 {
		return nil
	} else {
		_env_var_source := _env_var_sources[0].(map[string]interface{})
		return &api.EnvVarSource{
			FieldRef: createObjectFieldSelector(_env_var_source["field_ref"].([]interface{})),
		}
	}
}

func createObjectFieldSelector(_field_refs []interface{}) *api.ObjectFieldSelector {
	if len(_field_refs) == 0 {
		return nil
	} else {
		_field_ref := _field_refs[0].(map[string]interface{})
		return &api.ObjectFieldSelector{
			APIVersion: _field_ref["api_version"].(string),
			FieldPath:  _field_ref["field_path"].(string),
		}
	}
}

func createResourceRequirements(_resource_reqs []interface{}) *api.ResourceRequirements {
	if len(_resource_reqs) == 0 {
		return nil
	} else {
		_resource_req := _resource_reqs[0].(map[string]interface{})
		resource_req := &api.ResourceRequirements{}
		if val, ok := _resource_req["limits"]; ok {
			resource_req.Limits = createResourceList(val.(map[string]interface{}))
		}

		if val, ok := _resource_req["requests"]; ok {
			resource_req.Requests = createResourceList(val.(map[string]interface{}))
		}

		return resource_req
	}
}

func createResourceList(_resource_list map[string]interface{}) map[api.ResourceName]resource.Quantity {
	resource_list := make(map[api.ResourceName]resource.Quantity, len(_resource_list))
	for k, v := range(_resource_list) {
		if q, err := resource.ParseQuantity(v.(string)); err == nil && q != nil {
			resource_list[api.ResourceName(k)] = *q
		}
	}
	return resource_list
}

func createVolumeMounts(_volume_mounts []interface{}) []api.VolumeMount {
	volume_mounts := make([]api.VolumeMount, len(_volume_mounts))
	for i, v := range _volume_mounts {
		_volume_mount := v.(map[string]interface{})
		volume_mount := api.VolumeMount{
			MountPath: _volume_mount["mount_path"].(string),
		}

		if val, ok := _volume_mount["name"]; ok {
			volume_mount.Name = val.(string)
		}

		if val, ok := _volume_mount["read_only"]; ok {
			volume_mount.ReadOnly = val.(bool)
		}

		volume_mounts[i] = volume_mount
	}

	return volume_mounts
}

func createProbe(_probes []interface{}) *api.Probe {
	if len(_probes) == 0 {
		return nil
	} else {
		_probe := _probes[0].(map[string]interface{})
		probe := &api.Probe{}
		if val, ok := _probe["handler"]; ok {
			handler := createHandler(val.([]interface{}))
			probe.Exec = handler.Exec
			probe.HTTPGet = handler.HTTPGet
			probe.TCPSocket = handler.TCPSocket
		}

		if val, ok := _probe["initial_delay_seconds"]; ok {
			probe.InitialDelaySeconds = int64(val.(int))
		}

		if val, ok := _probe["timeout_seconds"]; ok {
			probe.TimeoutSeconds = int64(val.(int))
		}

		return probe
	}
}

func createHandler(_handlers []interface{}) *api.Handler {
	if len(_handlers) == 0 {
		return nil
	} else {
		_handler := _handlers[0].(map[string]interface{})
		handler := &api.Handler{}
		if val, ok := _handler["exec"]; ok {
			handler.Exec = createExecAction(val.([]interface{}))
		}

		if val, ok := _handler["http_get"]; ok {
			handler.HTTPGet = createHttpGetAction(val.([]interface{}))
		}

		if val, ok := _handler["tcp_socket"]; ok {
			handler.TCPSocket = createTcpSocketAction(val.([]interface{}))
		}

		return handler
	}
}

func createExecAction(_execs []interface{}) *api.ExecAction {
	if len(_execs) == 0 {
		return nil
	} else {
		_exec := _execs[0].(map[string]interface{})
		exec := &api.ExecAction{}
		exec.Command = createStringList(_exec["command"].([]interface{}))
		return exec
	}
}

func createHttpGetAction(_http_gets []interface{}) *api.HTTPGetAction {
	if len(_http_gets) == 0 {
		return nil
	} else {
		_http_get := _http_gets[0].(map[string]interface{})
		httpGet := &api.HTTPGetAction{}

		httpGet.Port = util.NewIntOrStringFromInt(_http_get["port"].(int))

		if val, ok := _http_get["path"]; ok {
			httpGet.Path = val.(string)
		}

		if val, ok := _http_get["host"]; ok {
			httpGet.Host = val.(string)
		}

		if val, ok := _http_get["scheme"]; ok {
			httpGet.Scheme = api.URIScheme(val.(string))
		}

		return httpGet
	}
}

func createTcpSocketAction(_tcp_sockets []interface{}) *api.TCPSocketAction {
	if len(_tcp_sockets) == 0 {
		return nil
	} else {
		_tcp_socket := _tcp_sockets[0].(map[string]interface{})
		tcpSocket := &api.TCPSocketAction{}

		tcpSocket.Port = util.NewIntOrStringFromInt(_tcp_socket["port"].(int))

		return tcpSocket
	}
}

func createLifecycle(_lifecycles []interface{}) *api.Lifecycle {
	if len(_lifecycles) == 0 {
		return nil
	} else {
		_lifecycle := _lifecycles[0].(map[string]interface{})
		return &api.Lifecycle {
			PostStart: createHandler(_lifecycle["post_start"].([]interface{})),
			PreStop: createHandler(_lifecycle["pre_stop"].([]interface{})),
		}
	}
}

func createSecurityContext(_security_contexts []interface{}) *api.SecurityContext {
	if len(_security_contexts) == 0 {
		return nil
	} else {
		_security_context := _security_contexts[0].(map[string]interface{})
		securityContext := &api.SecurityContext{}

		if val, ok := _security_context["capabilities"]; ok {
			securityContext.Capabilities = createCapabilities(val.([]interface{}))
		}

		if val, ok := _security_context["privileged"]; ok {
			b := val.(bool)
			securityContext.Privileged = &b
		}

		if val, ok := _security_context["se_linux_options"]; ok {
			securityContext.SELinuxOptions = createSeLinuxOptions(val.([]interface{}))
		}

		if val, ok := _security_context["run_as_user"]; ok {
			v := int64(val.(int))
			securityContext.RunAsUser = &v
		}

		if val, ok := _security_context["run_as_non_root"]; ok {
			securityContext.RunAsNonRoot = val.(bool)
		}

		return securityContext
	}
}

func createCapabilities(_capabilities []interface{}) *api.Capabilities {
	if len(_capabilities) == 0 {
		return nil
	} else {
		_capability := _capabilities[0].(map[string]interface{})
		capability := &api.Capabilities{}

		if val, ok := _capability["add"]; ok {
			capability.Add = createCapabilityList(val.([]interface{}))
		}

		if val, ok := _capability["drop"]; ok {
			capability.Drop = createCapabilityList(val.([]interface{}))
		}

		return capability
	}
}

func createCapabilityList(_values []interface{}) []api.Capability {
	values := make([]api.Capability, len(_values))
	for i, v := range _values {
		values[i] = api.Capability(v.(string))
	}
	return values
}

func createSeLinuxOptions(_se_linux_options []interface{}) *api.SELinuxOptions {
	if len(_se_linux_options) == 0 {
		return nil
	} else {
		_se_linux_option := _se_linux_options[0].(map[string]interface{})
		seLinuxOption := &api.SELinuxOptions{}

		if val, ok := _se_linux_option["user"]; ok {
			seLinuxOption.User = val.(string)
		}

		if val, ok := _se_linux_option["role"]; ok {
			seLinuxOption.Role = val.(string)
		}

		if val, ok := _se_linux_option["type"]; ok {
			seLinuxOption.Type = val.(string)
		}

		if val, ok := _se_linux_option["level"]; ok {
			seLinuxOption.Level = val.(string)
		}

		return seLinuxOption
	}
}
