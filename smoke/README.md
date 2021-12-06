# QA for OpenShift Shared Resource CSI Driver
This is a Behavior-driven development (or BDD) test framework leveraging `behave` for `python`. BDD is an agile software development technique that encourages collaboration between developers, QA and non-technical or business participants in a software project.Behave uses tests written in a natural language style, backed up by Python code.

We are using this framework to achieve the QA verification of any Jira Stories development takes on for the OpenShift Shared Resource CSI Driver, as well as any regression test suites the team wants to cultivate for the OpenShift Shared Resource CSI Driver.

Hence, the reader should consider these tests the 'next phase' of testing after the [golang based tests](https://github.com/openshift/csi-driver-shared-resource/tree/master/test/e2e) that run in OpenShift CI prow jobs as part of merging pull requests with code changes. These provide another means of testing that are focused on the end user experience, in language and terms that end users can understand.

Both QA and development have agreed to utilize this framework, and the description of test scenarios detailed below, as a replacement for direct communication between QA and development in OpenShift's internal polarion server.  QA will separately deal with taking the test scenarios descriptions here and importing into polarion for maintaining compatibility with the existing processes of the broader QA organization.

# Run smoke test on local machine
To run the smoke test on your local machine, run the below 3 commands on your terminal
```sh
$ oc login [cluster-server] -u username -p password --kubeconfig=kubeconfig
$ export KUBECONFIG=kubeconfig
$ make smoke
```
# Add Test Cases
### Behave operates on directories containing:

- feature files written by your Business Analyst / Sponsor / whoever with your behaviour scenarios in it.
- a “steps” directory with Python step implementations for the scenarios.

### The minimum requirement for a features directory is:
```
features/
features/everything.feature
features/steps/
features/steps/steps.py
```
### A more complex directory might look like:
```
features/
features/signup.feature
features/login.feature
features/account_details.feature
features/environment.py
features/steps/
features/steps/website.py
features/steps/utils.py
```
A feature file has a natural language format describing a feature or part of a feature with representative examples of expected outcomes. They’re plain-text (encoded in UTF-8) and look something like:
```gherkin
Feature: Fight or flight
  In order to increase the ninja survival rate,
  As a ninja commander
  I want my ninjas to decide whether to take on an
  opponent based on their skill levels

  Scenario: Weaker opponent
    Given the ninja has a third level black-belt
     When attacked by a samurai
     Then the ninja should engage the opponent

  Scenario: Stronger opponent
    Given the ninja has a third level black-belt
     When attacked by Chuck Norris
     Then the ninja should run for his life
```
## Python Step Implementations
Steps used in the scenarios are implemented in Python files in the “steps” directory. You can call these whatever you like as long as they use the python *.py file extension. You don’t need to tell behave which ones to use - it’ll use all of them.

If the test case captured by the feature file is something which QA needs to run manually vs. a set of python based steps, then the contributor stops at the introduction of the feature file, and QA/Dev review for correctness.

The full detail of the Python side of behave is in the [API documentation](https://behave.readthedocs.io/en/stable/api.html).

Steps are identified using [decorators](https://docs.python.org/3/search.html?q=decorator) which match the predicate from the feature file: given, when, then and step (variants with Title case are also available if that’s your preference.) The decorator accepts a string containing the rest of the phrase used in the scenario step it belongs to.

### Given a Scenario:
```gherkin
Scenario: Search for an account
   Given I search for a valid account
    Then I will see the account details
```
### Step code implementing the two steps here might look like (using selenium webdriver and some other helpers):
```python
@given('I search for a valid account')
def step_impl(context):
    context.browser.get('http://localhost:8000/index')
    form = get_element(context.browser, tag='form')
    get_element(form, name="msisdn").send_keys('61415551234')
    form.submit()

@then('I will see the account details')
def step_impl(context):
    elements = find_elements(context.browser, id='no-account')
    eq_(elements, [], 'account not found')
    h = get_element(context.browser, id='account-head')
    ok_(h.text.startswith("Account 61415551234"),
        'Heading %r has wrong text' % h.text)
```
Openshift dedicated python modules [openshift.py](https://github.com/openshift/csi-driver-shared-resource/tree/master/smoke/features/steps/openshift.py), [command.py](https://github.com/openshift/csi-driver-shared-resource/tree/master/smoke/features/steps/command.py), [project.py](https://github.com/openshift/csi-driver-shared-resource/tree/master/smoke/features/steps/project.py).These modules are used as a wrapper for openshift cli,one can create the Openshift object & use the functionalities to check the resources.
```sh
└── steps
    ├── command.py
    ├── openshift.py
    ├── project.py
```
Visit [Behave](https://behave.readthedocs.io/en/stable/tutorial.html) for more info.
# Test Results

The test results are JUnit files generated for each feature & are collected in `_output` dir post test run is complete
# Contribution Workflow
- For each feature QE/Dev to create a feature file inside the smoke/features directory, the feature file name should be synonymus with the feature being developed.
- Team to review the feature file and post approval QE to add the feature into polarion.
- Once the feature file is approved then the contibutor can go ahead and work on the step definitions for the feature. The step definition file(.py) needs to be in the smoke/features/steps/ directory.

#  Future Goals
- Integrate the smoke test into a QE specific CI that is triggered on nightly basis to carry out the QA for CSI shared resource driver.
- Automate the feature migration process into polarion post the feature is approved.

