pipeline {
    agent {
        docker {
            label 'main'
            image 'storjlabs/ci:latest'
            alwaysPull true
            args '-u root:root --cap-add SYS_PTRACE -v "/tmp/gomod":/go/pkg/mod -v "/tmp/npm":/npm --tmpfs "/tmp:exec,mode=777"'
        }
    }
    options {
        timeout(time: 40, unit: 'MINUTES')
        skipDefaultCheckout(true)
    }
    environment {
        NPM_CONFIG_CACHE = '/npm/cache'
        GOTRACEBACK = 'all'
        COCKROACH_MEMPROF_INTERVAL=0
    }
    stages {
        stage('Checkout') {
            steps {
                // Delete any content left over from a previous run.
                sh "chmod -R 777 ."
                // Bash requires extglob option to support !(.git) syntax,
                // and we don't want to delete .git to have faster clones.
                sh 'bash -O extglob -c "rm -rf !(.git)"'

                checkout scm

                sh 'mkdir -p .build'

                // make a backup of the mod file, because sometimes they get modified by tools
                // this allows to lint the unmodified files
                sh 'cp go.mod .build/go.mod.orig'
                sh 'cp testsuite/go.mod .build/testsuite.go.mod.orig'

                // download dependencies
                sh 'go mod download'
                sh 'cd testsuite && go mod download'

                // pre-check that we cannot do at a later stage reliably
                sh 'check-large-files'

                // add a stub go.mod file to node projects to prevent go build ./... from scanning them
                sh 'mkdir -p web/satellite/node_modules web/storagenode/node_modules web/multinode/node_modules'
                sh 'touch web/satellite/node_modules/go.mod web/storagenode/node_modules/go.mod web/multinode/node_modules/go.mod'
            }
        }
        stage('Build') {
            parallel {
                stage('go') {
                    steps {
                        // use go test to build all the packages, including tests
                        sh 'go test -v -p 16 -bench XYZXYZXYZXYZ -run XYZXYZXYZXYZ ./...'
                    }
                }
                stage('go -race') {
                    steps {
                        // use go test to build all the packages, including tests
                        sh 'go test -v -p 16 -bench XYZXYZXYZXYZ -run XYZXYZXYZXYZ -race ./...'

                        // install storj-sim
                        sh 'go install -race -v storj.io/storj/cmd/satellite '+
                                'storj.io/storj/cmd/storagenode ' +
                                'storj.io/storj/cmd/storj-sim ' +
                                'storj.io/storj/cmd/versioncontrol ' +
                                'storj.io/storj/cmd/uplink ' +
                                'storj.io/storj/cmd/identity ' +
                                'storj.io/storj/cmd/certificates ' +
                                'storj.io/storj/cmd/multinode'
                    }
                }
                stage('go -race gateway') {
                    steps {
                        // install gateway for storj-sim
                        sh 'go install -race -v storj.io/gateway@latest'
                    }
                }

                stage('db') {
                    steps {
                        sh 'service postgresql start'
                        dir('.build') {
                            sh 'cockroach start-single-node --insecure --store=type=mem,size=2GiB --listen-addr=localhost:26256 --http-addr=localhost:8086 --cache 512MiB --max-sql-memory 512MiB --background'
                            sh 'cockroach start-single-node --insecure --store=type=mem,size=2GiB --listen-addr=localhost:26257 --http-addr=localhost:8087 --cache 512MiB --max-sql-memory 512MiB --background'
                            sh 'cockroach start-single-node --insecure --store=type=mem,size=2GiB --listen-addr=localhost:26258 --http-addr=localhost:8088 --cache 512MiB --max-sql-memory 512MiB --background'
                            sh 'cockroach start-single-node --insecure --store=type=mem,size=2GiB --listen-addr=localhost:26259 --http-addr=localhost:8089 --cache 512MiB --max-sql-memory 512MiB --background'
                            sh 'cockroach start-single-node --insecure --store=type=mem,size=2GiB --listen-addr=localhost:26260 --http-addr=localhost:8090 --cache 256MiB --max-sql-memory 256MiB --background'
                        }
                    }
                }

                stage('web/satellite') {
                    steps {
                        dir('web/satellite') {
                            sh 'npm ci --prefer-offline --no-audit'
                            sh './scripts/build-wasm.sh'
                            sh 'npm run build'
                        }
                    }
                }
                stage('web/storagenode') {
                    steps {
                        dir('web/storagenode') {
                            sh 'npm ci --prefer-offline --no-audit'
                            sh 'npm run build'
                        }
                    }
                }
                stage('web/multinode') {
                    steps {
                        dir('web/multinode') {
                            sh 'npm ci --prefer-offline --no-audit'
                            sh 'npm run build'
                        }
                    }
                }
                stage('satellite/admin/ui') {
                    steps {
                        dir('satellite/admin/ui') {
                            sh 'npm ci --prefer-offline --no-audit'
                            sh 'npm run build'
                            sh 'rm -rf build .svelte-kit' // Remove these directories for avoiding linting their files.
                        }
                    }
                }
            }
        }

        stage('Verification') {
            parallel {
                stage('Lint') {
                    steps {
                        sh 'check-copyright'
                        sh 'check-imports -race ./...'
                        sh 'check-peer-constraints -race'
                        sh 'check-atomic-align ./...'
                        sh 'check-monkit ./...'
                        sh 'check-errs ./...'
                        sh 'staticcheck ./...'
                        sh 'golangci-lint --config /go/ci/.golangci.yml -j=2 run'
                        sh 'check-mod-tidy -mod .build/go.mod.orig'
                        sh 'make check-monitoring'
                        sh 'make test-wasm-size'

                        sh 'protolock status'

                        dir("testsuite") {
                            sh 'check-imports ./...'
                            sh 'check-atomic-align ./...'
                            sh 'check-monkit ./...'
                            sh 'check-errs ./...'
                            sh 'staticcheck ./...'
                            sh 'golangci-lint --config /go/ci/.golangci.yml -j=2 run'
                            sh 'check-mod-tidy -mod ../.build/testsuite.go.mod.orig'
                        }

                        dir("satellite/admin/ui") {
                            sh 'npm run check'
                            sh 'npm run lint'
                        }

                        sh './scripts/check-package-lock.sh'
                    }
                }
                stage('Cross Compile') {
                    steps {
                        // verify most of the commands, we cannot check everything since some of them
                        // have a C dependency and we don't have cross-compilation in storj/ci image
                        sh 'GOOS=linux   GOARCH=386   go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=linux   GOARCH=amd64 go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=linux   GOARCH=arm   go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=linux   GOARCH=arm64 go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=windows GOARCH=386   go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=windows GOARCH=amd64 go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=windows GOARCH=arm64 go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=darwin  GOARCH=amd64 go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                        sh 'GOOS=darwin  GOARCH=arm64 go vet ./cmd/uplink ./cmd/satellite ./cmd/storagenode-updater ./cmd/storj-sim'
                    }
                }
                stage('Tests') {
                    environment {
                        STORJ_TEST_COCKROACH = 'cockroach://root@localhost:26256/testcockroach?sslmode=disable;' +
                            'cockroach://root@localhost:26257/testcockroach?sslmode=disable;' +
                            'cockroach://root@localhost:26258/testcockroach?sslmode=disable;' +
                            'cockroach://root@localhost:26259/testcockroach?sslmode=disable'
                        STORJ_TEST_COCKROACH_ALT = 'cockroach://root@localhost:26260/testcockroach?sslmode=disable'
                        STORJ_TEST_POSTGRES = 'postgres://postgres@localhost/teststorj?sslmode=disable'
                        STORJ_TEST_LOG_LEVEL = 'info'
                        COVERFLAGS = "${ env.BRANCH_NAME == 'main' ? '-coverprofile=.build/coverprofile -coverpkg=storj.io/storj/private/...,storj.io/storj/satellite/...,storj.io/storj/storage/...,storj.io/storj/storagenode/...,storj.io/storj/versioncontrol/...' : ''}"
                    }
                    steps {
                        sh 'cockroach sql --insecure --host=localhost:26256 -e \'create database testcockroach;\''
                        sh 'cockroach sql --insecure --host=localhost:26257 -e \'create database testcockroach;\''
                        sh 'cockroach sql --insecure --host=localhost:26258 -e \'create database testcockroach;\''
                        sh 'cockroach sql --insecure --host=localhost:26259 -e \'create database testcockroach;\''
                        sh 'cockroach sql --insecure --host=localhost:26260 -e \'create database testcockroach;\''

                        sh 'cockroach sql --insecure --host=localhost:26259 -e \'create database testmetabase;\''

                        sh 'psql -U postgres -c \'create database teststorj;\''
                        sh 'psql -U postgres -c \'create database testmetabase;\''
                        sh 'use-ports -from 1024 -to 10000 &'

                        sh 'go test -parallel 4 -p 6 -vet=off $COVERFLAGS -timeout 32m -json -race ./... 2>&1 | tee .build/tests.json | xunit -out .build/tests.xml'
                    }

                    post {
                        always {
                            archiveArtifacts artifacts: '.build/tests.json'
                            sh script: 'cat .build/tests.json | tparse -all -top -slow 100', returnStatus: true
                            junit '.build/tests.xml'

                            script {
                                if(fileExists(".build/coverprofile")){
                                    sh script: 'filter-cover-profile < .build/coverprofile > .build/clean.coverprofile', returnStatus: true
                                    sh script: 'gocov convert .build/clean.coverprofile > .build/cover.json', returnStatus: true
                                    sh script: 'gocov-xml  < .build/cover.json > .build/cobertura.xml', returnStatus: true
                                    cobertura coberturaReportFile: '.build/cobertura.xml',
                                        lineCoverageTargets: '70, 60, 50',
                                        autoUpdateHealth: false,
                                        autoUpdateStability: false,
                                        failUnhealthy: true
                                }
                            }
                        }
                    }
                }

                stage('Check Benchmark') {
                    environment {
                        STORJ_TEST_COCKROACH = 'omit'
                        STORJ_TEST_POSTGRES = 'postgres://postgres@localhost/benchstorj?sslmode=disable'
                    }
                    steps {
                        sh 'psql -U postgres -c \'create database benchstorj;\''
                        sh 'go test -parallel 1 -p 1 -vet=off -timeout 20m -short -run XYZXYZXYZXYZ -bench . -benchtime 1x ./...'
                    }
                }

                stage('Integration') {
                    environment {
                        // use different hostname to avoid port conflicts
                        STORJ_NETWORK_HOST4 = '127.0.0.2'
                        STORJ_NETWORK_HOST6 = '127.0.0.2'

                        STORJ_SIM_POSTGRES = 'postgres://postgres@localhost/teststorj2?sslmode=disable'
                    }

                    steps {
                        sh 'psql -U postgres -c \'create database teststorj2;\''
                        sh 'make test-sim'

                        // sh 'make test-certificates' // flaky
                    }
                }

                stage('Cockroach Integration') {
                    environment {
                        STORJ_NETWORK_HOST4 = '127.0.0.4'
                        STORJ_NETWORK_HOST6 = '127.0.0.4'

                        STORJ_SIM_POSTGRES = 'cockroach://root@localhost:26257/testcockroach4?sslmode=disable'
                    }

                    steps {
                        sh 'cockroach sql --insecure --host=localhost:26257 -e \'create database testcockroach4;\''
                        sh 'make test-sim'
                        sh 'cockroach sql --insecure --host=localhost:26257 -e \'drop database testcockroach4;\''
                    }
                }

                stage('Integration Redis unavailability') {
                    environment {
                        // use different hostname to avoid port conflicts
                        STORJ_NETWORK_HOST4 = '127.0.0.6'
                        STORJ_NETWORK_HOST6 = '127.0.0.6'
                        STORJ_REDIS_PORT = '7379'

                        STORJ_SIM_POSTGRES = 'postgres://postgres@localhost/teststorj6?sslmode=disable'
                    }

                    steps {
                        sh 'psql -U postgres -c \'create database teststorj6;\''
                        sh 'make test-sim-redis-unavailability'
                    }
                }

                stage('Backwards Compatibility') {
                    environment {
                        STORJ_NETWORK_HOST4 = '127.0.0.3'
                        STORJ_NETWORK_HOST6 = '127.0.0.3'

                        STORJ_SIM_POSTGRES = 'postgres://postgres@localhost/teststorj3?sslmode=disable'
                        STORJ_MIGRATION_DB = 'postgres://postgres@localhost/teststorj3?sslmode=disable&options=--search_path=satellite/0/meta'
                    }

                    steps {
                        sh 'psql -U postgres -c \'create database teststorj3;\''
                        sh 'make test-sim-backwards-compatible'
                    }
                }

                stage('Cockroach Backwards Compatibility') {
                    environment {
                        STORJ_NETWORK_HOST4 = '127.0.0.5'
                        STORJ_NETWORK_HOST6 = '127.0.0.5'

                        STORJ_SIM_POSTGRES = 'cockroach://root@localhost:26257/testcockroach5?sslmode=disable'
                        STORJ_MIGRATION_DB = 'postgres://root@localhost:26257/testcockroach5/satellite/0/meta?sslmode=disable'
                    }

                    steps {
                        sh 'cockroach sql --insecure --host=localhost:26257 -e \'create database testcockroach5;\''
                        sh 'make test-sim-backwards-compatible'
                        sh 'cockroach sql --insecure --host=localhost:26257 -e \'drop database testcockroach5;\''
                    }
                }

                stage('wasm npm') {
                    steps {
                        dir(".build") {
                            sh 'cp -r ../satellite/console/wasm/tests/ .'
                            sh 'cd tests && cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" .'
                            sh 'cd tests && npm install && npm run test'
                        }
                    }
                }

                stage('web/satellite') {
                    steps {
                        dir("web/satellite") {
                            sh 'npm run lint-ci'
                            sh script: 'npm audit', returnStatus: true
                            sh 'npm run test'
                        }
                    }
                }

                stage('web/storagenode') {
                    steps {
                        dir("web/storagenode") {
                            sh 'npm run lint-ci'
                            sh script: 'npm audit', returnStatus: true
                            sh 'npm run test'
                        }
                    }
                }

                stage('web/multinode') {
                    steps {
                        dir("web/multinode") {
                            sh 'npm run lint-ci'
                            sh script: 'npm audit', returnStatus: true
                            sh 'npm run test'
                        }
                    }
                }

                stage('satellite/admin/ui') {
                    steps {
                        dir("satellite/admin/ui") {
                            sh script: 'npm audit', returnStatus: true
                        }
                    }
                }
            }
        }
/*
        stage('UI') {
            when {
                anyOf {
                    branch 'main'
                    branch pattern: "release-.*", comparator: "REGEXP"
                    changeset "testsuite/ui/**"
                    changeset "web/**"
                    changeset "satellite/console/**"
                    changeset "storagenode/console/**"
                    changeset "multinode/console/**"
                }
            }
            environment {
                STORJ_TEST_COCKROACH = 'omit'
                STORJ_TEST_POSTGRES = 'postgres://postgres@localhost/testui?sslmode=disable'
                STORJ_TEST_BROWSER  = '/usr/bin/chromium'
                STORJ_TEST_SATELLITE_WEB = "${pwd()}/web/satellite"
                STORJ_TEST_EDGE_HOST = '127.0.0.10'
            }
            steps {
                sh 'psql -U postgres -c \'create database testui;\''
                catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE') {
                    sh 'cd testsuite && go test -parallel 1 -p 1 -short -vet=off -timeout 5m -json -race ./... 2>&1 | tee ../.build/ui-tests.json | xunit -out ../.build/ui-tests.xml'
                }
            }
            post {
                always {
                    archiveArtifacts artifacts: '.build/ui-tests.json'
                    sh script: 'cat .build/ui-tests.json | tparse -all -top -slow 100', returnStatus: true
                    junit '.build/ui-tests.xml'
                }
            }
        }
*/
        stage('Post') {
            parallel {
                stage('Lint') {
                    steps {
                        sh 'check-clean-directory'
                    }
                }
            }
        }
    }
}