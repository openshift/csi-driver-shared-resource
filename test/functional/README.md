# Functional test for OpenShift Shared Resource CSI Driver

This is a Behavior-driven development (or BDD) test framework leveraging behave for python. BDD is an agile software development technique that encourages collaboration between developers, QA and non-technical or business participants in a software project.Behave uses tests written in a natural language style, backed up by Python code.

We are using this to test the functionality of Openshift Shared Resource CSI Driver

# Steps to run on local machine

- pip3  install -r /test/functional/requirements.txt
- export KUBECONFIG = path/to/kubeconfig
- cd test
- behave functional/features --show-source


