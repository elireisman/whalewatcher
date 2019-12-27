# whalewatcher

Have you ever used Docker Compose to manage your local dev or CI environment's backing data stores, and found it tricky to automate interactions with those stores due to difficulty determining when the containers are ready to process data? `depends_on` or `curl`ing a liveness endpoint will suffice in the simple cases, but in many cases the store doesn't supply a simple, reliable solution. Coordinating such dependencies is a headache itself, and scripting checks for heterogenous systems in your pipeline can be ponderous and error-prone.

`whalewatcher` will monitor each specified containers' logs for a regex pattern you specify, exposing an HTTP endpoint dependent containers or external services in your project can use to determine readiness of all target containers, or any subset of interest. (Setup is easy)[./docker-compose.yml]. The endpoint returns granular JSON-formatted status info as well as aggregate status via the HTTP status code in the response. Calling services can determine with confidence when all of their service dependencies are ready to perform work, minus the boilerplate and setup complexity.

## Demo
If you have Docker and Docker Compose installed locally, you can run `make demo` to see a service waiting on `whalewatcher` in action, using the included example Compose file.

Other `make` targets of interest: `build`, `docker`, `example`, `test` and `clean`.

## API
Processes that block on `whalewatcher` status can reach the service a number of ways. The examples below assume the configuration in the supplied `docker-compose.yml`:
- Internal (within Docker Compose network, from container context):
  - `curl http://example-whalewatcher:4444/` to view status for _all_ configured target containers
  - `curl http://example-whalewatcher:4444/?status=example-kafka,example-elasticsearch` to view status for selected targets only
- External (from host machine using an externally mapped port):
  - `curl http://localhost:4444/` to view status for _all_ configured target containers
  - `curl http://localhost:4444/?status=example-kafka,example-elasticsearch` to view status for selected targets only

### Aggregate Status
HTTP status codes are used to return aggregate readiness info for all configured targets, or the subset specified in the caller's request. Are we abusing HTTP status codes for convenience here? Probably. I'll let you be the judge.

| Code         | Meaning                                        |
| ------------ | -----------------                              |
| 200          | All services ready                             |
| 202          | Some services not ready yet - continue polling |
| 404          | The requested service(s) are not configured    |
| 500          | Internal error, check Compose and config files |
| 503          | One or more target services in fatal error     |

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
- Mount the `docker.sock` as shown in the exmaple, or otherwise configure the Docker API env vars for your client
- Mount the `whalewatcher` YAML config file or supply the YAML inline as shown in the example (see below for details)
- Expose a port of your choice and direct callers to block, pinging the endpoint until `whalewatcher` responds with a `200 OK`

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

