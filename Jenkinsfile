@Library('jenkins-helpers@v0.1.19') _

def label = "victoriametrics-1-${UUID.randomUUID().toString()}"

podTemplate(
    label: label,
    annotations: [
        podAnnotation(key: "jenkins/build-url", value: env.BUILD_URL ?: ""),
        podAnnotation(key: "jenkins/github-pr-url", value: env.CHANGE_URL ?: ""),
    ],
    containers: [containerTemplate(name: 'docker',
                                   command: '/bin/cat -',
                                   image: 'eu.gcr.io/cognitedata/docker-image:v18.09.8-3147306',
                                   resourceRequestCpu: '200m',
                                   resourceRequestMemory: '300Mi',
                                   resourceLimitCpu: '200m',
                                   resourceLimitMemory: '300Mi',
                                   ttyEnabled: true)],
    volumes: [secretVolume(secretName: 'jenkins-docker-builder',
                           mountPath: '/jenkins-docker-builder'),
              hostPathVolume(hostPath: '/var/run/docker.sock', mountPath: '/var/run/docker.sock')]) {
    node(label) {
        def gitCommit
        container('jnlp') {
            stage('Checkout') {
                checkout(scm)
                gitCommit = sh(returnStdout: true, script: 'git rev-parse --short HEAD').trim()
            }
        }
        def tag = "v${gitCommit}"
        container('docker') {
            stage('Build VictoriaMetrics binaries') {
                sh("make vminsert-prod vmselect-prod vmstorage-prod")
            }
            stage('Build VictoriaMetrics Docker image') {
                sh("make package")
            }
            if(env.BRANCH_NAME != "master") {
                echo "Not in master branch. Will not push image. Skip."
                return
            }
            stage('Push Docker image') {
                sh('#!/bin/sh -e\n' + 'docker login -u _json_key -p "$(cat /jenkins-docker-builder/credentials.json)" https://eu.gcr.io')
                sh("docker push eu.gcr.io/cognitedata/victoriametrics/vmselect:${tag}")
                sh("docker push eu.gcr.io/cognitedata/victoriametrics/vminsert:${tag}")
                sh("docker push eu.gcr.io/cognitedata/victoriametrics/vmstorage:${tag}")
            }
        }
    }
}
