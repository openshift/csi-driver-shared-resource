# @mark.steps
# ----------------------------------------------------------------------------
# STEPS:
# ----------------------------------------------------------------------------
import os
import time
import urllib3
from behave import given, when, then
from pyshould import should
from kubernetes import config, client
from functional.features.steps.openshift import Openshift
from functional.features.steps.project import Project



# [WIP]Test results file path
# scripts_dir = os.getenv('OUTPUT_DIR')

# variables needed to get the resource status
current_project = ''
config.load_kube_config()
oc = Openshift()

def triggerbuild(buildconfig,namespace):
    print('Triggering build: ',buildconfig)
    res = oc.start_build(buildconfig,namespace)
    print(res)

# STEP
@given(u'Project "{project_name}" is used')
def given_project_is_used(context, project_name):
    project = Project(project_name)
    global current_project
    current_project = project_name
    context.current_project = current_project
    context.oc = oc
    if not project.is_present():
        print("Project is not present, creating project: {}...".format(project_name))
        project.create() | should.be_truthy.desc(
            "Project {} is created".format(project_name))
    print("Project {} is created!!!".format(project_name))
    context.project = project


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