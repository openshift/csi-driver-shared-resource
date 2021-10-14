#!/bin/sh

shout() {
  set +x
  echo -e "\n!!!!!!!!!!!!!!!!!!!!\n${1}\n!!!!!!!!!!!!!!!!!!!!\n"
  set -x
}

#-----------------------------------------------------------------------------
# Global Variables
#-----------------------------------------------------------------------------
export V_FLAG=-v
OUTPUT_DIR="$(pwd)"/_output
export OUTPUT_DIR
export LOGS_DIR="${OUTPUT_DIR}"/logs
export GOLANGCI_LINT_BIN="${OUTPUT_DIR}"/golangci-lint
export PYTHON_VENV_DIR="${OUTPUT_DIR}"/venv3
# -- Variables for smoke tests
export TEST_SMOKE_ARTIFACTS=/tmp/artifacts

# -- Setting up the venv
python3 -m venv "${PYTHON_VENV_DIR}"
"${PYTHON_VENV_DIR}"/bin/pip install --upgrade setuptools
"${PYTHON_VENV_DIR}"/bin/pip install --upgrade pip

mkdir -p "${LOGS_DIR}"/smoke-tests-logs
mkdir -p "${OUTPUT_DIR}"/smoke-tests-output
touch "${OUTPUT_DIR}"/backups.txt
TEST_SMOKE_OUTPUT_DIR="${OUTPUT_DIR}"/smoke
export TEST_SMOKE_OUTPUT_DIR

echo "Logs directory created at ""${LOGS_DIR}"/smoke

# -- Trigger the test
shout "Environment setup in progress"
"${PYTHON_VENV_DIR}"/bin/pip install -q -r smoke/requirements.txt
shout "Running smoke tests"
echo "Logs will be collected in ""${TEST_SMOKE_OUTPUT_DIR}"
"${PYTHON_VENV_DIR}"/bin/behave --junit --junit-directory "${TEST_SMOKE_OUTPUT_DIR}" \
                              --no-capture --no-capture-stderr \
                              --tags="~manual" smoke/features                     
echo "Logs collected in ""${TEST_SMOKE_OUTPUT_DIR}"

shout "cleanup test projects"

set +x

for i in $(oc projects -q); do
    if [[ $i == "testing-namespace"* ]]; then
        oc delete project $i
    fi
done