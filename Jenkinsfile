@Library('jenkins-helpers@fix-spin-access-error') _


dockerUtils.pod() {
  baker.pod() {
    spinnaker.pod() {
      node(POD_LABEL) {
        checkout(scm)
        deploySpinnakerPipelineConfigs.upload()
      }
    }  
  }
} 