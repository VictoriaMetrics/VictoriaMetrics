from datadog import initialize, statsd

import os

from flask import Flask

options = {
    "statsd_host": "127.0.0.1",
    "statsd_port": 8125,
}

initialize(**options)

app = Flask(__name__)

@app.route('/')
def hello_world():
    statsd.gauge('active.connections', 1001, tags=["protocol:http"])
    target = os.environ.get('TARGET', 'World')
    return 'Hello {}!\n'.format(target)

if __name__ == "__main__":
    app.run(debug=True,host='0.0.0.0',port=int(os.environ.get('PORT', 8080)))
