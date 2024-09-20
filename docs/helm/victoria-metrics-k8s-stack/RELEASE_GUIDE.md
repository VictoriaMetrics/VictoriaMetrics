# Release process guidance 

## Update version for VictoriaMetrics kubernetes monitoring stack

1. Update dependency requirements in [Chart.yml](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/Chart.yaml)
2. Apply changes via `helm dependency update`
3. Update image tag in chart values:

    <div class="with-copy" markdown="1">
    
    ```console
    make sync-rules
    make sync-dashboards
    ```
    </div>
4. Bump version of the victoria-metrics-k8s-stack [Chart.yml](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/Chart.yaml)
5. Run linter:

    <div class="with-copy" markdown="1">
    
    ```console
    make lint
    ```
    
    </div>
6. Render templates locally to check for errors: 
    
    <div class="with-copy" markdown="1">
    
    ```console
    helm template vm-k8s-stack ./charts/victoria-metrics-k8s-stack --output-dir out --values ./charts/victoria-metrics-k8s-stack/values.yaml --debug
    ```
    
    </div>
7. Test updated chart by installing it to your kubernetes cluster.
8. Update docs with
    ```console
    helm-docs
    ```
9.  Commit the changes and send a [PR](https://github.com/VictoriaMetrics/helm-charts/pulls)
