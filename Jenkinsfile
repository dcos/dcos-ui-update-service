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
    stage("Run Tests") {
      parallel {
        stage("Format") {
          steps {
            sh 'test -z "$(gofmt -l -d ./ | tee /dev/stderr)"'
          }
        }

        stage("Lint") {
          steps {
            sh 'golint -set_exit_status ./'
          }
        }

        // TODO: take a look coverages (https://github.com/dcos/dcos-go/blob/master/scripts/test.sh#L56)
        stage("Unit Test") {
          steps {
            sh 'go test -v ./...'
          }
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