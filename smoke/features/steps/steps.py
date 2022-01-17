# @mark.steps
# ----------------------------------------------------------------------------
# STEPS:
# ----------------------------------------------------------------------------

import os
import re
import yaml

# Will be needed in future
# import json
# import time
# import urllib3

from behave import given, then, when
from kubernetes import client, config
from pyshould import should
from smoke.features.steps.openshift import Openshift
from smoke.features.steps.project import Project
from smoke.features.steps.generic import Util
from smoke.features.steps.command import Command

# Test results file path
scripts_dir = os.getenv('OUTPUT_DIR')

# variables needed to get the resource status
global current_project
current_project = ''
config.load_kube_config()
oc = Openshift()
util = Util()
cmd = Command()

def edit_resource(context, share_resource, new_data, path, resource):
    print(f"start editing resource {share_resource}")
    util.edit_resource_yaml_file(path, new_data, resource)
    update_cmd = path + " -n " + current_project
    oc.oc_apply(update_cmd)

# STEP
@given(u'Project "{project_name}" is used')
def given_project_is_used(context, project_name):
    project = Project(project_name)
    current_project = project_name
    context.current_project = current_project
    context.oc = oc
    if not project.is_present():
        print("Project is not present, creating project: {}...".format(project_name))
        project.create() | should.be_truthy.desc(
            "Project {} is created".format(project_name))
    print("Project {} is created!!!".format(project_name))
    context.project = project


def before_feature(context, feature):
    if scenario.name != None and "TEST_NAMESPACE" in scenario.name:
        print("Scenario using env namespace subtitution found: {0}, env: {}".format(scenario.name, os.getenv("TEST_NAMESPACE")))
        scenario.name = txt.replace("TEST_NAMESPACE", os.getenv("TEST_NAMESPACE"))

# STEP
@given(u'Project [{project_env}] is used')
def given_namespace_from_env_is_used(context, project_env):
    env = os.getenv(project_env)
    assert env is not None, f"{project_env} environment variable needs to be set"
    print(f"{project_env} = {env}")
    given_project_is_used(context, env)
    
@given(u'We have an openshift techpreview cluster')
def loginCluster(context):
    print("Using [{}]".format(current_project))
    featureSet = "TechPreviewNoUpgrade"
    cmd = "get featuregate cluster -o json | jq \'.spec.featureSet\'"
    output = oc.execute_command(cmd)
    if not re.search(r'.*%s' % featureSet, output):
        featureFile = "./smoke/features/data/featuregate.yaml"
        oc.oc_apply(featureFile)

@given(u'shared resource csi driver is installed')
def check_shared_resource_driver(context):
    namespace = "openshift-cluster-csi-drivers"
    project = Project(namespace)
    project.namespace_exist(namespace)
    cmd = "get ds -n {namespace}"
    output = oc.execute_command(cmd)
    output.find("shared-resource-csi-driver-node")
    print("Shared resource driver is installed")

@given(u'user has cluster scoped level permission to create CRD "{share_resource}"')
def share_resource_scope(context, share_resource):
    print("verifying user permission for crd '{}'".format(share_resource))
    oc.crd_permission(share_resource)

@when(u'user creates the configmap "{share_config}" in a given namespace')
def configmap(context, share_config):
    namespace = Project().current_project()
    print(f"creating configmap {share_config} within namespace: {namespace}")
    oc.oc_apply("./smoke/features/data/configmap.yaml")

@then(u'create a project')
def setProject(context):  
    namespace = "testing-namespace" + util.random_string(4, 2)
    project = Project(namespace)
    print("create a new project: {}".format(namespace))
    project.create_namespace(namespace)

@when(u'defines shared configmap {my_shared_config} that references the {shared_config} configmap from the first project to share across all namespace')
def shared_configmap(context, my_shared_config, shared_config):
    namespace = Project().current_project()
    cmfile = "./smoke/features/data/shareconfigmap.sh"
    oc.shell_cmd(cmfile, namespace)

@when(u'creates another project that will access the cluster scoped shared configmap that references the configmap in the first project')
def another_project(context):
    current_project = Project().current_project() 
    setProject(context)

@when(u'RBAC for the service account to use the {shared_resource} in its pod')
def create_rbac(context, shared_resource): 
    rbacFile = "./smoke/features/data/rbac.sh"
    oc.shell_cmd(rbacFile, shared_resource)

@when(u'creates a pod {pod_name} with a CSI volume citing the shared resource csi driver and requesting the previously defined {shared_resource} in the Pod CSI volume\'s volume attributes')
def create_pod(context, pod_name, shared_resource):
    namespace = Project().current_project()
    print(f"creating pod {pod_name} within namespace: {namespace}")
    podFile = "./smoke/features/data/pod.sh"
    oc.shell_cmd(podFile, shared_resource)

@when(u'edits configMap {share_config} data {value} from the first project')
def edit_configmap_with_data(context, share_config, value):
    path = "./smoke/features/data/configmap.yaml"
    new_data = {
        value: ''
    }
    edit_resource(context, share_config, new_data, path, share_config)

@then(u'pod {pod_name} in the second project should reflect the change {data} to the {share_resource}')
def pod_log_contains(context, pod_name, data, share_resource):
    match = oc.get_pod_log(pod_name, data)
    if match:
        print(f"pod {pod_name} successfully reflect the changes")

@when(u'user creates the secret {my_secret} in a given namespace')
def create_secret(context, my_secret):
    namespace = Project().current_project()
    print(f"creating secret {my_secret} within namespace: {namespace}")
    oc.oc_apply("./smoke/features/data/secret.yaml")

@when(u'defines shared secret {my_shared_secret} that references the {my_secret} secret from the first project to share across all namespace')
def shared_secret(context, my_shared_secret, my_secret):
    namespace = Project().current_project()
    secretfile = "./smoke/features/data/sharesecret.sh"
    oc.shell_cmd(secretfile, namespace)
        
@when(u'edits secret {my_secret} data from the first project')
def edit_secret(context, my_secret):
    new_data = {
        'stringData': {
            'hostname': 'quay.io'
        }
    }
    path = "./smoke/features/data/secret.yaml"
    edit_resource(context, my_secret, new_data, path, my_secret)
    # print(f"start editing secret {my_secret}")
    # util.edit_yaml_file(path, new_data)
    # update_cmd = path + " -n " + current_project
    # oc.oc_apply(update_cmd)

@when(u'user adds {refresh_Resources} to {value} in {share_config} configmap')
def add_refresh_resource(context, refresh_Resources, value, share_config):
    new_data = {
        "config.yaml": "---\nrefreshResources:  false\n"
    }
    path = "./smoke/features/data/configmap.yaml"
    edit_resource(context, share_config, new_data, path, share_config)

@then(u'pod {pod_name} in the second project should not reflect the change {data} to the {share_resource}')
def pod_log_not_contains(context, pod_name, data, share_resource):
    match = oc.get_pod_log(pod_name, data)
    if not match:
        print(f"pod {pod_name} do not reflect changes as refreshResource is disabled")

