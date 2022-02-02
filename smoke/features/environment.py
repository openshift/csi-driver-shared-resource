'''
before_step(context, step), after_step(context, step)
    These run before and after every step.
    The step passed in is an instance of Step.
before_feature(context, feature), after_feature(context, feature)
    These run before and after each feature file is exercised.
    The feature passed in is an instance of Feature.
before_all(context), after_all(context)
    These run before and after the whole shooting match.
'''

import subprocess
from pyshould import should
from smoke.features.steps.openshift import Openshift
from smoke.features.steps.project import Project

'''
before_scenario(context, scenario), after_scenario(context, scenario)
    These run before and after each scenario is run.
    The scenario passed in is an instance of Scenario.
'''

oc = Openshift()

def before_feature(context, feature):
    global first_project
    print("Running feature file: {0}".format(feature.name))

def before_scenario(context, scenario):
    print("\nGetting OC status before {} scenario".format(scenario))
    code, output = subprocess.getstatusoutput('oc get project default')
    print("[CODE] {}".format(code))
    print("[CMD] {}".format(output))
    code | should.be_equal_to(0)
    print("***Connected to cluster***")

def after_scenario(context, scenario):
    print("\nclean up steps for scenario {}".format(scenario))
    print("\ndelete the created shared resources")
    share_resource = {'sharedconfigmap':'my-shared-config', 'sharedsecret':'my-shared-secret'}
    for resource, name in share_resource.items():
        output = oc.is_resource_in(resource)
        if output:
            oc.delete(resource, name, "default")
    print("\ndelete the namespace created for the scenario")
    project = Project().get_all_project()
    output = project.splitlines()
    for i in output:
        if "testing-namespace" in i:
            Project().delete_namespace(i)