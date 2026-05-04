# Location Service

The location service polls a configurable provider for the vehicle position, resolves an IANA timezone from that position, exposes the latest state over HTTP, and can keep the service schedule timezone and Pi system timezone aligned.

## Provider

The default provider is `rutx50`, backed by the Teltonika RUTX50 JSON GPS position endpoint supplied by the local router:

```text
http://192.168.51.1/api/gps/position/status
```

The endpoint is configurable because the router address can differ between networks.

Example config:

```yaml
location:
  enabled: true
  provider: rutx50
  poll_interval: 5m
  rutx50:
    endpoint: http://192.168.51.1/api/gps/position/status
    login_endpoint: http://192.168.51.1/api/login
    username: admin
    password_file: /var/lib/xtura/rutx50-password
    insecure_skip_verify: false
    timeout: 10s
    # auth_token: optional bearer token if the endpoint is protected
```

If `auth_token` is set, the provider sends it directly as `Authorization: Bearer <token>`. Otherwise, when `password` or `password_file` is configured, it logs in with `POST /api/login`, caches the returned token, and retries login once after a `401`. The adapter first tries the RutOS REST payload shape `{"data":{"username":"...","password":"..."}}`, then falls back to the legacy top-level shape `{"username":"...","password":"..."}` if the router rejects the first request as unauthorized or malformed.

Prefer `password_file` over putting the router password directly in `/var/lib/xtura/config.yaml`:

```bash
sudo install -o xtura -g xtura -m 0600 /dev/stdin /var/lib/xtura/rutx50-password
```

The provider expects a JSON response containing latitude and longitude fields. It accepts common key names such as `latitude`/`longitude`, `lat`/`lon`, and nested objects. On the tested RUTX50, `GET /api/gps/status` returned only service metadata (`uptime` and `dpo_support`), while `GET /api/gps/position/status` returned `latitude` and `longitude`.

## Movement Signal

The service infers a coarse `is_moving` signal from GPS displacement. It keeps recent fixes in memory and sums the distance between consecutive fixes inside a configurable window:

```yaml
location:
  movement:
    window: 15m
    min_distance_meters: 250
```

With the default `poll_interval: 5m`, this gives a movement signal that is intentionally accurate over minutes rather than seconds. It is meant for automation rules such as disabling equipment while the vehicle is moving, not for displaying live driving speed. The state also includes `movement_meters`, the cumulative displacement currently used for the decision.

## Timezone Lookup

By default, coordinates are resolved locally with [`github.com/ringsaturn/tzf`](https://github.com/ringsaturn/tzf). This avoids relying on an external timezone API while the vehicle is travelling.

```yaml
location:
  timezone:
    provider: tzf
```

The `tzf` default finder uses a simplified dataset and may be imperfect right on timezone borders, but it is local and fast. The upstream project notes that `NewDefaultFinder` trades some border accuracy for speed and memory use; `NewFullFinder` is available upstream if full-precision lookup becomes necessary.

GeoTimeZone is still available as an explicit HTTP resolver:

```text
https://api.geotimezone.com/public/timezone
```

That service returns an `iana_timezone` field, for example `Europe/London` or `Europe/Paris`. The lookup can be disabled with:

```yaml
location:
  timezone:
    provider: none
```

When timezone lookup is disabled, the location state reports the current configured automation timezone.

## Timezone Updates

The service can update `/var/lib/xtura/config.yaml` so the heating schedule timezone follows the GPS-derived timezone:

```yaml
location:
  timezone_update:
    enabled: true
    interval: 1h
    update_config: true
```

To also update the Raspberry Pi system timezone, configure a command. The service appends the resolved timezone name as the final argument:

```yaml
location:
  timezone_update:
    enabled: true
    interval: 1h
    update_config: true
    command: ["sudo", "/usr/bin/timedatectl", "set-timezone"]
```

The systemd service runs as `xtura`. The deploy script installs [ops/sudoers/xtura-timezone](../ops/sudoers/xtura-timezone), which permits that user to run `/usr/bin/timedatectl set-timezone *` without a password. The systemd unit intentionally does not set `NoNewPrivileges=true`, otherwise `sudo` cannot elevate for this narrow command. Without that permission, location polling still works, but the state will include the command error in `last_error`.

## API

`GET /v1/location/state` returns the latest location state:

```json
{
  "configured": true,
  "known": true,
  "provider": "rutx50",
  "latitude": 51.5007,
  "longitude": -0.1246,
  "is_moving": true,
  "movement_meters": 410.2,
  "timezone": "Europe/London",
  "system_timezone": "Europe/London",
  "last_updated_at": "2026-05-03T18:30:00Z",
  "timezone_update_mode": "config"
}
```

When no successful poll has completed yet, `known` is `false` and `last_error` may describe the most recent failure.
