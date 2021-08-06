#! /bin/bash

set -e
set -o pipefail

# Run e2e tests. The e2e tests assume that the desired driver has been deployed to the cluster
# Environment variables used for configuration:
#
# TEST_SUITE: test suite to run. Defaults to "normal", can also be "disruptive" and "slow"
# TEST_TIMEOUT: timeout for tests. Defauls to 30m, can be any parsable duration.

suite=${TEST_SUITE:-"normal"}
timeout=${TEST_TIMEOUT:-"30m"}

function test() {
    echo "Running e2e suite ${suite}"
    KUBERNETES_CONFIG=${KUBECONFIG} go test -race -count 1 -tags "${suite}" -timeout "${timeout}" -v "./test/e2e/..."
}

test
