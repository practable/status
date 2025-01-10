# status
Status server interacts with the book, jump, and relay services to identify experiments with (un)healthy streams and adjusts the status of experiment resources in booking system accordingly:

| All required streams ok? | Jump OK? | Available in booking system? |
|---|---|---|
| Y | X | Y| 
| N | X | N|


Key: **Y**: yes, **N**: no, **X**: does not matter

Email alerts are sent on status changes to both jump and streams.

| Stream health changed? | Jump health changed? | Send Email? | Change availabilty? |
|---|---|---|---|
| N | N | N| N |
| N | Y | Y| N |
| Y | N | Y| Y |
| Y | Y | Y| Y |

An experiment without a jump connection can still be used so long as the required streams are present, although it is an issue for the maintainer to handle, so email alerts are sent for changes to jump and streams health events (an event is a change in the health).

### Criteria

#### Jump

Jump connections remain dormant with no communication, until a client wishes to log in. At which point, there is traffic recorded in the `rx` section of the statistics. A healthy jump connection is simply one that exists - there is no need to check the stats.

#### Relay

Healthy stream connections from an experiment have transmitted recently, typically every few seconds or more frequently. Rather than define a healthy stream, as this requires some quite detailed checks, we can instead detect definitely unhealthy streams and alert on those. An unhealthy stream is defined as NOT having transmitted wtihin the last 10s. 


## Usage

Status requires configuration with the details of the book, jump and relay services, an optional email relay, and some tuning parameters that control how quick it is to adjust the status of experiments.

Typically, `status` is configured using [admin tools](https://github.com/practable/admin-tools) scripts for your instance, where it is usually started as a `systemd` service. 

A local version can be run using `./scripts/local-demo.sh` which will need modifying to suit your configuration.

```
export STATUS_BASEPATH_BOOK=/tenant/book
export STATUS_BASEPATH_JUMP=/tenant/jump
export STATUS_BASEPATH_RELAY=/tenant/relay
export STATUS_EMAIL_CC="beta@a.org"
export STATUS_EMAIL_FROM=other@b.org
export STATUS_EMAIL_HOST=stmp.b.org
export STATUS_EMAIL_LINK=app.practable.io/tenant/status
export STATUS_EMAIL_PASSWORD=something
export STATUS_EMAIL_PORT=587
export STATUS_EMAIL_SUBJECT=app.practable.io/tenant
export STATUS_EMAIL_TO="alpha@a.org"
export STATUS_HEALTH_LAST=10s
export STATUS_HEALTH_LOG_EVERY=10m
export STATUS_HEALTH_EVENTS=100
export STATUS_HEALTH_STARTUP=1m
export STATUS_HOST_BOOK=app.example.org
export STATUS_HOST_JUMP=app.example.org
export STATUS_HOST_RELAY=app.example.org
export STATUS_LOG_LEVEL=warn
export STATUS_LOG_FORMAT=json
export STATUS_LOG_FILE=/var/log/status/status.log
export STATUS_PORT_PROFILE=6061
export STATUS_PORT_SERVE=3007
export STATUS_PROFILE=false
export STATUS_QUERY_BOOK_EVERY=15m
export STATUS_RECONNECT_JUMP_EVERY=1d
export STATUS_RECONNECT_RELAY_EVERY=1d
export STATUS_SCHEME_BOOK=https
export STATUS_SCHEME_JUMP=https
export STATUS_SCHEME_RELAY=https
export STATUS_SECRET_JUMP=othersecret
export STATUS_SECRET_BOOK=somesecret
export STATUS_SECRET_RELAY=somesecret
export STATUS_TIMEOUT_BOOK=5s
status serve
```

### Tuning

The tuning parameters affect the responsiveness of the system to problems, and how complete a history of issues is stored, both of which must be traded off against CPU, memory and disk usage. Some suggested ranges are 

| parameter | test | prod | comment |
|---|---|---|---|
|STATUS_HEALTH_LAST |10s | 10s | healthy stream must have last transmitted within this duration. Configure experiments with a heartbeat for data streams that is shorter than this duration. |
|STATUS_HEALTH_LOG_EVERY | 1m | 10m-1h | how often to write current overall status info to log file |
|STATUS_HEALTH_EVENTS | 20 | 20 | how many health events to store in memory per experiment 
|STATUS_HEALTH_STARTUP | 1m | 5m | how long to wait after an experiment has first connected to before assessing overall health | 
|STATUS_QUERY_BOOK_EVERY |5m | 15m-1h | how often to check the booking manifest for required streams |
|STATUS_RECONNECT_JUMP_EVERY | 30s | 1d-30d | Reconnection is automatic, this just provides a way to have a long running service without setting a long token lifetime up front |
|STATUS_RECONNECT_RELAY_EVERY |30s | 1d-30d | " |
|STATUS_TIMEOUT_BOOK |5s | 30s | This is how long to wait for GET/PUT requests to the book server |

## Email 

Many cloud providers do not allow outgoing access to `smtp` (port 25), so as to reduce spam. Many email providers offer an `smtp` service running on port 587, with basic auth. If your current provider does not offer this, it is relatively inexpensive to obtain a custom domain and then seek an inexpensive email provider to host an account for that domain.

### Multiple addresses
Multiple addresses are supported - just put emails in a comma-separated list

### Rate limits
Emails can only be sent as often as checks are performed, and only one email is sent per check, summarising all the health events that just occurred, as well as listing all current problems in a single list (to aid cut and paste forwarding of the current state info to other team members, for eample). Rate limits on emails are therefore enforced by the frequency with which relay and jump send status updates. If both relay and jump were to fail, then the 


## Trouble shooting 

If you are seeing unexpected errors, it is likely a configuration issue. The openapi client code can be difficult to develop with because unexpected errors tend to throw an unmarshalling error rather than passing the server's error message. Hence some clients used by status are written using raw HTTP requests. If you are seeing unexpected errors, then it could well be a configuration error that is being masked by one of the clients that uses the openapi client code. In these cases, check the logs for the services to see whether something straightforward like a token generation error is preventing authentication, or whether requests are misdirected (e.g. incorrect base path such as using the wrong instance/tenant sub path). Note that `book` and `jump` currently require `/api/v1` in the base path but `relay` does not.

## Development tools

```
go install github.com/go-swagger/go-swagger/cmd/swagger@latest
```