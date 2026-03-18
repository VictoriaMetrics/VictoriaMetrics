
  # Pass vmalert flags (extraArgs become CLI flags)[web:57]
  extraArgs:
    envflag.enable: true
    envflag.prefix: VM_
    httpListenAddr: :8880
    loggerFormat: json
    # Chart writes server.config.alerts into /config/alert-rules.yaml by default[web:57]
    rule:
      - /config/alert-rules.yaml

    # Make Alertmanager "Source" button link back to Grafana Explore[web:34][web:51]
    external.url: http://127.0.0.1:3000
    external.alert.source: >
      explore?orgId=1&left={"datasource":"victoriametrics",
      "queries":[{"refId":"A","expr":"{{.Expr|queryEscape}}"}]}

