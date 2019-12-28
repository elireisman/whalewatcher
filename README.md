## The Problem

Docker and Docker Compose are great for managing services your projects depend on, especially in local dev or CI environments. However, it's often tricky to determine the readiness of containerized services when dependencies are involved. Tools like [this](https://github.com/vishnubob/wait-for-it) are great for the simple cases, but lots of open source software doesn't accommodate this approach and can cause friction in a multi-stage Compose project.

Writing one-off checks for a set of heterogenous services is error prone, and the Docker folks [kinda punt on this](https://docs.docker.com/compose/startup-order/) for code you didn't write. At best, your utility image will require a CLI client for each service your code depends on. At worst, you will resort to peppering sleeps into your services' `entrypoint`s or `command` clauses and crossing your fingers. A one-size-fits-all solution we can drop into our Docker Compose projects would be ideal here.


## The Solution?

`whalewatcher` monitors the `docker log`s of a set of target containers for regex patterns you specify. When a match is found, `whalewatcher` exposes the target's ready status via an API callers can poll. Dependent containers and/or external services can use `whalewatcher` to determine when a set of target containers are ready to perform work. By making use of the `status` URL query parameter, multiple services in your project with distinct dependencies can achieve reliable multi-stage warmup utilizing a single instance of `whalewatcher`.

Adding `whalewatcher` to your Docker or Docker Compose based project is (pretty straightforward)[./docker-compose.yml], but see below for the juicy details.

#### Disclaimer:
`whalewatcher` is a quick-and-dirty solution for your local dev/CI needs, and isn't intended for Docker Swarm, Kubernetes, or production environments at present. Linux and OSX are verified. Verification on Windows I leave as an exercise for the reader, but the libraries backing `whalewatcher` are Windows compatible so...?


## Demo

Requirements:
 - `make`, `docker`, and `docker-compose` installed locally
 - exec one of the `make` targets listed below, `CTRL-C` to exit

| `make` Target | Action Taken             |
| ------------- | ------------------------ |
| demo          | runs `docker-compose`, `curl`s `whalewatcher` from host machine to demo an external service monitoring status of services it depends on |
| internal-demo | runs `docker-compose`, `curl`s `whalewatcher` from `watcher` service to demo a containerized service monitoring status of services it depends on |
| example       | runs `docker-compose`, tails the logs from the `whalewatcher` container itself to provide an under-the-hood view of what it does |
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
  - `curl -sS -o /dev/null -w '%{http_code}' http://localhost:5555/?status=demo-mysql,demo-redis` to view aggregate status only, for selected targets

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
  "demo-elasticsearch": {
    "ready": false,
    "at": "2019-06-19T12:15:33.1721458Z",
    "error": "java.io.FileNotFoundException: /var/run/elasticsearch/elasticsearch.pid (No such file or directory)"
  },
  "demo-kafka": {
    "ready": true,
    "at": "2019-06-19T12:13:01.1721561Z",
    "error": ""
  }
  "demo-mongodb": {
    "ready": false
    "error": ""
  }
}
```


## Setup

### Add to your project
- Add a service using the `initialcontext/whalewatcher:latest` [image](https://hub.docker.com/repository/docker/initialcontext/whalewatcher) to your `docker-compose.yml`
- Configure the Docker API client for your `whalewatcher` service (choose one):
    - Mount the host `docker.sock` as shown in the example Compose file
    - Configure the env vars for the [API client](https://godoc.org/github.com/docker/docker/client)
- Configure the `whalewatcher` container instance (see below for details)
- Ensure all the `service`s you will monitor set a `container_name: <NAME>` attribute
- Direct dependent services to poll `whalewatcher` for readiness status on containers of interest

### Configure the tool
`whalewatcher` is configured using a YAML file and some CLI arguments. Each entry in the `containers` clause should be keyed using the `container_name` of the service to be monitored. The `pattern` attribute is used to supply a regex pattern to match a log line indicating the monitored service is ready.

#### Example config file
```
containers:
  container_name_one:
    pattern: 'regex pattern one'
  container_name_two:
    pattern: 'regex pattern two'
  # ...and so on...
```

#### CLI arguments
Arguments you can supply to `whalewatcher` directly:

| Argument | Default | Example | Description |
| -------- | ------- | ------- | ----------- |
| `--config-path` | "/etc/whalewatcher/config.yaml" | "./whalewatcher.yaml" | Path to YAML service config file |
| `--config-var`  | "" | "SOME_ENV_VAR" | If set, the name of the env var in which the YAML config is inlined |
| `--wait-millis` | 60000 | 10000 | Time in millis `whalewatcher` should await each target container before signalling failure |
| `--port`        | 4444 | 5432 | the port `whalewatcher` should listen on to expose the status API |


## Contributing
`whalewatcher` has successfully reduced my annoyance and stress levels setting up a new project with Docker Compose in my dev/CI env, but feels pretty "alpha" still. PR's welcome!

