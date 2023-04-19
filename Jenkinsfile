#!groovy

@Library(['github.com/cloudogu/ces-build-lib@1.64.1'])
import com.cloudogu.ces.cesbuildlib.*

// Creating necessary git objects
git = new Git(this, "cesmarvin")
git.committerName = 'cesmarvin'
git.committerEmail = 'cesmarvin@cloudogu.com'
gitflow = new GitFlow(this, git)
github = new GitHub(this, git)
changelog = new Changelog(this)
Docker docker = new Docker(this)
gpg = new Gpg(this, docker)

// Configuration of repository
repositoryOwner = "cloudogu"
repositoryName = "k8s-service-discovery"
project = "github.com/${repositoryOwner}/${repositoryName}"

// Configuration of branches
productionReleaseBranch = "main"
developmentBranch = "develop"
currentBranch = "${env.BRANCH_NAME}"

node('docker') {
    timestamps {
        stage('Checkout') {
            checkout scm
            make 'clean'
        }

        stage('Lint') {
            lintDockerfile()
        }

        stage('Check Markdown Links') {
            Markdown markdown = new Markdown(this, "3.11.0")
            markdown.check()
        }

        docker
                .image('golang:1.20.3')
                .mountJenkinsUser()
                .inside("--volume ${WORKSPACE}:/go/src/${project} -w /go/src/${project}")
                        {
                            stage('Build') {
                                make 'build-controller'
                            }

                            stage('k8s-Integration-Test') {
                                make 'k8s-integration-test'
                            }

                            stage("Review dog analysis") {
                                stageStaticAnalysisReviewDog()
                            }

                            stage('Generate k8s Resources') {
                                make 'create-temporary-release-resources'
                                archiveArtifacts 'target/make/k8s/*.yaml'
                            }
                        }

        stage("Lint k8s Resources") {
            stageLintK8SResources()
        }

        stage('SonarQube') {
            stageStaticAnalysisSonarQube()
        }

        K3d k3d = new K3d(this, "${WORKSPACE}", "${WORKSPACE}/k3d", env.PATH)

        try {
            Makefile makefile = new Makefile(this)
            String controllerVersion = makefile.getVersion()

            stage('Set up k3d cluster') {
                k3d.startK3d()
            }

            def imageName
            stage('Build & Push Image') {
                imageName=k3d.buildAndPushToLocalRegistry("cloudogu/${repositoryName}", controllerVersion)
            }

            GString sourceDeploymentYaml="target/make/k8s/${repositoryName}_${controllerVersion}.yaml"
            GString sourceDeploymentYamlWithNamespace="target/make/k8s/${repositoryName}_${controllerVersion}_namespaced.yaml"

            stage('Update development resources') {
                sh "cat ${sourceDeploymentYaml} | sed \"s/{{ .Namespace }}/default/\" > ${sourceDeploymentYamlWithNamespace}"
                docker.image('mikefarah/yq:4.22.1')
                        .mountJenkinsUser()
                        .inside("--volume ${WORKSPACE}:/workdir -w /workdir") {
                            sh "yq -i '(select(.kind == \"Deployment\").spec.template.spec.containers[]|select(.name == \"manager\")).image=\"${imageName}\"' ${sourceDeploymentYamlWithNamespace}"
                        }
            }

            stage('Deploy Manager') {
                k3d.kubectl("apply -f ${sourceDeploymentYamlWithNamespace}")
            }

            stage('Wait for Ready Rollout') {
                k3d.kubectl("--namespace default wait --for=condition=Ready pods --all")
            }

            stageAutomaticRelease()
        } finally {
            stage('Remove k3d cluster') {
                k3d.deleteK3d()
            }
        }
    }
}

void gitWithCredentials(String command) {
    withCredentials([usernamePassword(credentialsId: 'cesmarvin', usernameVariable: 'GIT_AUTH_USR', passwordVariable: 'GIT_AUTH_PSW')]) {
        sh(
                script: "git -c credential.helper=\"!f() { echo username='\$GIT_AUTH_USR'; echo password='\$GIT_AUTH_PSW'; }; f\" " + command,
                returnStdout: true
        )
    }
}

void stageLintK8SResources() {
    String kubevalImage = "cytopia/kubeval:0.13"
    Makefile makefile = new Makefile(this)
    String controllerVersion = makefile.getVersion()

    docker
            .image(kubevalImage)
            .inside("-v ${WORKSPACE}/target/make/k8s:/data -t --entrypoint=")
                    {
                        sh "kubeval /data/${repositoryName}_${controllerVersion}.yaml --ignore-missing-schemas"
                    }
}

void stageStaticAnalysisReviewDog() {
    def commitSha = sh(returnStdout: true, script: 'git rev-parse HEAD').trim()

    withCredentials([[$class: 'UsernamePasswordMultiBinding', credentialsId: 'sonarqube-gh', usernameVariable: 'USERNAME', passwordVariable: 'REVIEWDOG_GITHUB_API_TOKEN']]) {
        withEnv(["CI_PULL_REQUEST=${env.CHANGE_ID}", "CI_COMMIT=${commitSha}", "CI_REPO_OWNER=cloudogu", "CI_REPO_NAME=${repositoryName}"]) {
            make 'static-analysis-ci'
        }
    }
}

void stageStaticAnalysisSonarQube() {
    def scannerHome = tool name: 'sonar-scanner', type: 'hudson.plugins.sonar.SonarRunnerInstallation'
    withSonarQubeEnv {
        sh "git config 'remote.origin.fetch' '+refs/heads/*:refs/remotes/origin/*'"
        gitWithCredentials("fetch --all")

        if (currentBranch == productionReleaseBranch) {
            echo "This branch has been detected as the production branch."
            sh "${scannerHome}/bin/sonar-scanner -Dsonar.branch.name=${env.BRANCH_NAME}"
        } else if (currentBranch == developmentBranch) {
            echo "This branch has been detected as the development branch."
            sh "${scannerHome}/bin/sonar-scanner -Dsonar.branch.name=${env.BRANCH_NAME}"
        } else if (env.CHANGE_TARGET) {
            echo "This branch has been detected as a pull request."
            sh "${scannerHome}/bin/sonar-scanner -Dsonar.pullrequest.key=${env.CHANGE_ID} -Dsonar.pullrequest.branch=${env.CHANGE_BRANCH} -Dsonar.pullrequest.base=${developmentBranch}"
        } else if (currentBranch.startsWith("feature/")) {
            echo "This branch has been detected as a feature branch."
            sh "${scannerHome}/bin/sonar-scanner -Dsonar.branch.name=${env.BRANCH_NAME}"
        } else {
            echo "This branch has been detected as a miscellaneous branch."
            sh "${scannerHome}/bin/sonar-scanner -Dsonar.branch.name=${env.BRANCH_NAME} "
        }
    }
    timeout(time: 2, unit: 'MINUTES') { // Needed when there is no webhook for example
        def qGate = waitForQualityGate()
        if (qGate.status != 'OK') {
            unstable("Pipeline unstable due to SonarQube quality gate failure")
        }
    }
}

void stageAutomaticRelease() {
    if (gitflow.isReleaseBranch()) {
        String releaseVersion = git.getSimpleBranchName()
        String dockerReleaseVersion = releaseVersion.split("v")[1]

        stage('Build & Push Image') {
            def dockerImage = docker.build("cloudogu/${repositoryName}:${dockerReleaseVersion}")
            docker.withRegistry('https://registry.hub.docker.com/', 'dockerHubCredentials') {
                dockerImage.push("${dockerReleaseVersion}")
            }
        }

        stage('Finish Release') {
            gitflow.finishRelease(releaseVersion, productionReleaseBranch)
        }

        stage('Push to Registry') {
            Makefile makefile = new Makefile(this)
            String controllerVersion = makefile.getVersion()
            GString targetOperatorResourceYaml = "target/make/k8s/${repositoryName}_${controllerVersion}.yaml"

            DoguRegistry registry = new DoguRegistry(this)
            registry.pushK8sYaml(targetOperatorResourceYaml, repositoryName, "k8s", "${controllerVersion}")
        }

        stage('Add Github-Release') {
            releaseId = github.createReleaseWithChangelog(releaseVersion, changelog, productionReleaseBranch)
        }
    }
}

void make(String makeArgs) {
    sh "make ${makeArgs}"
}