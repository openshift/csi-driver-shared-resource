# @mark.steps
# ----------------------------------------------------------------------------
# STEPS:
# ----------------------------------------------------------------------------

import os

# Will be needed in future
# import json
# import time
# import urllib3

from behave import given, then, when
from kubernetes import client, config
from pyshould import should
from smoke.features.steps.openshift import Openshift
from smoke.features.steps.project import Project

# Test results file path
scripts_dir = os.getenv('OUTPUT_DIR')

# variables needed to get the resource status
deploy_pod = "jenkins-1-deploy"
global jenkins_master_pod
global current_project
current_project = ''
config.load_kube_config()
oc = Openshift()
podStatus = {}

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
    
@given(u'we have a openshift tech-preview cluster')
def loginCluster(context):
    print("Using [{}]".format(current_project))