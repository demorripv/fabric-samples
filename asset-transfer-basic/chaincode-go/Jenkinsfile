pipeline {
    agent any

    parameters {
        string(name: 'REMOTE_URL', defaultValue: '', description: 'The remote repository URL')
        string(name: 'COMMIT_HASH', defaultValue: '', description: 'The commit hash')
    }

    stages {
        stage('Trigger Builds') {
            
                stage('Build on Guix1') {
                    steps {
                        script {
                            def result = httpRequest(
                                url: "http://192.168.1.118:8080/build",
                                httpMode: 'POST',
                                requestBody: groovy.json.JsonOutput.toJson([
                                    remoteURL: params.REMOTE_URL,
                                    commitHash: params.COMMIT_HASH
                                ])
                            )
                            def response = readJSON text: result.content
                            env.HASH1 = response.hash
                        }
                    }
                }
                
                 
                    
            }
            
        }
         
}

