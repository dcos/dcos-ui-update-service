#!/usr/bin/env groovy

@Library("sec_ci_libs@v2-latest") _

def master_branches = ["master", ] as String[]

pipeline {
  agent {
    dockerfile {
      filename  "Dockerfile.dev"
    }
  }

  environment {
    JENKINS_VERSION = "yes"
  }

  options {
    timeout(time: 1, unit: "HOURS")
    disableConcurrentBuilds()
  }

  stages {
    stage("Build") {
      steps {
        sh 'GOOS=linux GO111MODULE=on go build ./...'
      }
    }

    stage("Run Tests") {
      parallel {
        stage("Lint") {
          steps {
            sh 'gometalinter --config=.gometalinter.json ./...'
          }
        }

        stage("Unit Test") {
          steps {
            sh 'go test -cover -coverprofile=coverage.txt -v ./...'
            sh 'gocover-cobertura < coverage.txt > coverage.xml'
          }
          post {
            always {
              archiveArtifacts "coverage.*"
            }
          }
        }
      }      
    }

    stage("Deploy to S3") {
      when {
        expression {
          master_branches.contains(BRANCH_NAME)
        }
      }

      steps {
        withCredentials([
          string(credentialsId: "1ddc25d8-0873-4b6f-949a-ae803b074e7a", variable: "AWS_ACCESS_KEY_ID"),
          string(credentialsId: "875cfce9-90ca-4174-8720-816b4cb7f10f", variable: "AWS_SECRET_ACCESS_KEY"),
        ]) {
          sh "GOOS=linux GO111MODULE=on go build -o build/dcos-ui-update-service ./"
          // TODO: think about versioning https://jira.mesosphere.com/browse/DCOS-43672
          sh "aws s3 cp build/dcos-ui-update-service s3://downloads.mesosphere.io/dcos-ui-update-service/latest/dcos-ui-update-service-${env.GIT_COMMIT} --acl public-read"
        }
      }
    }
  }

  post {
    failure {
      withCredentials([
        string(credentialsId: "8b793652-f26a-422f-a9ba-0d1e47eb9d89", variable: "SLACK_TOKEN")
      ]) {
        slackSend (
          channel: "#frontend-ci-status",
          color: "danger",
          message: "FAILED\nBranch: ${env.CHANGE_BRANCH}\nJob: <${env.RUN_DISPLAY_URL}|${env.JOB_NAME} [${env.BUILD_NUMBER}]>",
          teamDomain: "mesosphere",
          token: "${env.SLACK_TOKEN}",
        )
      }
    }
    unstable {
      withCredentials([
        string(credentialsId: "8b793652-f26a-422f-a9ba-0d1e47eb9d89", variable: "SLACK_TOKEN")
      ]) {
        slackSend (
          channel: "#frontend-ci-status",
          color: "warning",
          message: "UNSTABLE\nBranch: ${env.CHANGE_BRANCH}\nJob: <${env.RUN_DISPLAY_URL}|${env.JOB_NAME} [${env.BUILD_NUMBER}]>",
          teamDomain: "mesosphere",
          token: "${env.SLACK_TOKEN}",
        )
      }
    }
  }
}
