pipeline {
    agent any
    stages {
        stage('Build') {
            steps {
                checkout([$class: 'GitSCM', branches: [[name: '**']], doGenerateSubmoduleConfigurations: false, extensions: [[$class: 'RelativeTargetDirectory', relativeTargetDir: 'src'], [$class: 'CleanBeforeCheckout']], submoduleCfg: [], userRemoteConfigs: [[url: 'https://github.com/openshift-metalkube/coredns-mdns']]])
                checkout([$class: 'GitSCM', branches: [[name: '**']], doGenerateSubmoduleConfigurations: false, extensions: [[$class: 'RelativeTargetDirectory', relativeTargetDir: 'mdns-ci'], [$class: 'CleanBeforeCheckout']], submoduleCfg: [], userRemoteConfigs: [[url: 'https://github.com/cybertron/mdns-ci']]])
                lock('mdns') {
                    sh label: '', script: '${WORKSPACE}/mdns-ci/coredns-mdns-integration coredns-mdns'
                }
            }
        }
    }
}
