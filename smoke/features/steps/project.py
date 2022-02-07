import re
import os

from smoke.features.steps.command import Command
class Project():
    def __init__(self, name=''):
        self.name = name
        self.cmd = Command()

    def create(self):
        create_project_output, exit_code = self.cmd.run("oc new-project {}".format(self.name))
        if re.search(r'Now using project \"%s\"\son\sserver' % self.name, create_project_output) is not None or \
                re.search(r'.*Already\son\sproject\s\"%s\"\son\sserver.*' % self.name, create_project_output) is not None:
            return True
        elif re.search(r'.*project.project.openshift.io\s\"%s\"\salready exists' % self.name, create_project_output) is not None:
            return self.switch_to()
        else:
            print(f"Unexpected value returned '{create_project_output}'")
        return False

    def is_present(self):
        output, exit_code = self.cmd.run('oc get ns {}'.format(self.name))
        return exit_code == 0

    def current_project(self):
        output, exit_code = self.cmd.run(f'oc project -q')
        return output
    
    def get_all_project(self):
        output, exit_code = self.cmd.run(f'oc projects -q')
        return output

    def switch_to(self):
        create_project_output, exit_code = self.cmd.run('oc project {}'.format(self.name))
        if re.search(r'Now using project \"%s\"\son\sserver' % self.name, create_project_output) is not None:
            return True
        elif re.search(r'.*Already\son\sproject\s\"%s\"\son\sserver.*' % self.name, create_project_output) is not None:
            return True
        else:
            print(f"Unexpected project creating output: '{create_project_output}'")
        return False

    def namespace_exist(self, namespace):
        output, exit_status = self.cmd.run(f'oc get projects | grep {namespace}')
        return exit_status == 0

    def create_namespace(self, namespace):
        output, exit_status = self.cmd.run(f'oc new-project {namespace}')
        output.find(r'.*Now\susing\sproject\"%s\"\son\sserver.*' % {namespace})
        return exit_status == 0

    def delete_namespace(self, namespace):
        output, exit_status = self.cmd.run(f'oc delete project {namespace}')
        output.find(r'.*\"%s\"\sdeleted.*' % {namespace})
        return exit_status == 0
