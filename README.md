# whalewatcher

`whalewatcher` monitors the `docker log`s of a set of target containers for regex patterns you specify. When a match is found, `whalewatcher` exposes the target's ready status via a JSON API callers can poll. It exposes an HTTP endpoint and simple staus API that dependent containers and/or external services can use to determine when a set of target containers are ready to perform work. Using `whalewatcher` in your Docker Compose based project is (pretty straightforward)[./docker-compose.yml], but see below for the juicy details. It's meant as a quick-and-dirty solution for your local dev/CI needs only, so please use as directed.

## Demo
Requirements:
 - `make`, `docker`, and `docker-compose` installed locally
 - exec one of the `make` targets listed below, `CTRL-C` to exit

| `make` Target | Action Taken             |
| ------------- | ------------------------ |
| demo          | builds whalewatcher, runs `docker-compose`, `curl`s `whalewatcher` from host machine to demo an external service awaiting dependent services |
| internal-demo | builds `whalewatcher`, runs `docker-compose`, `curl`s `whalewatcher` from `watcher` service demo a containerized service awaiting dependent services |
| example       | builds `whalewatcher`, runs `docker-compose`, tails the logs from the `whalewatcher` container itself to provide more detailed in-flight view |
| clean         | removes built binaries and locally cached `whalewatcher` images, shuts down and cleans up `docker-compose` demo services |
| build         | builds the `whalewatcher` binary locally on the host |
| docker        | builds the `whalewatcher:latest` Docker image locally |

## API
Processes that block on `whalewatcher` status can reach the service a number of ways. The examples below assume the configuration in the supplied `docker-compose.yml`:
- Internal (within Docker Compose network, from container context):
  - `curl -sS http://demo-whalewatcher:4444/` to view status for _all_ configured target containers
  - `curl -sS http://demo-whalewatcher:4444/?status=demo-kafka,demo-elasticsearch` to view status for selected targets only
  - `curl -sS -o /dev/null -w '%{http_code}' http://demo-whalewatcher:4444/` to view aggregate status only, for all targets
- External (from host machine using an externally mapped port):
  - `curl -sS http://localhost:5555/` to view status for _all_ configured target containers
  - `curl -sS http://localhost:5555/?status=demo-zookeeper,demo-mysql,demo-mongodb` to view status for selected targets only
  - `curl -sS -o /dev/null -w '%{http_code}' http://localhost:5555/` to view aggregate status only, for all targets

### Aggregate Status
HTTP status codes are used to return aggregate readiness info for all configured targets, or the subset specified in the caller's request. Are we abusing HTTP status codes for convenience here? Probably. I'll let you be the judge.

| Status Code  | Meaning           |
| ------------ | ----------------- |
| 200          | All services ready for action |
| 202          | Some services not ready yet, please continue polling |
| 404          | The requested service(s) are not configured in `whalewatcher`  |
| 500          | Internal error, check your Compose and config files and error logs |
| 503          | One or more target services experienced a fatal error, start over   |

### Detailed Status
In addition, responses from `whalewatcher` will include a JSON body with a detailed status for each requested service:

```
{
  "example-elasticsearch": {
    "ready": false,
    "error": "Jun 19 12:15:33 my.es.server.net elasticsearch[1234]: java.io.FileNotFoundException: /var/run/elasticsearch/elasticsearch.pid (No such file or directory)"
  },
  "example-kafka": {
    "ready": true,
    "error": ""
  }
}
```

## Setup

### Add to your project
- Add a service using the `whalewatcher:latest` image to your `docker-compose.yml`
- Mount the `docker.sock` as shown in the example Compose file, or configure the Docker API env vars for your client
- Configure the `whalewatcher` container instance (see below for details)
- Direct dependent services to poll `whalewatcher` for readiness status on containers of interest

### Configure the tool
`whalewatcher` is configured using YAML. Users can supply the configuration inline in an environment var using the `--config-var <NAME>` argument, or by mounting a YAML file into the container and supplying the `--config-file <PATH>` argument. Each entry in the `containers` clause should be keyed using the `container_name` of the services to be monitored. Add a `container_name: <NAME>` entry to each clause in your Docker Compose if absent.

Example format:
```
containers:
  container_name_one:
    pattern: 'regex pattern one'
  container_name_two:
    pattern: 'regex pattern two'
  # ...and so on...
```

In addition, you can supply CLI arguments to override the port `whalewatcher` listens on, and the amount of time (in milliseconds) `whalewatcher` will await each target container's startup before log monitoring begins. Exceeding this timeout marks the target as a failed launch, which can be helpful in the event of a config typo or other upstream problems with your Docker Compose setup.

## Contributing
`whalewatcher` has successfully reduced my annoyance and stress levels setting up a new data project with Docker Compose in my dev/CI env, but feels pretty "alpha" still. There's lots to do to make it more flexible, robust, and simple to set up and use. PR's welcome!

