# LLAMA

LLAMA (Loss and LAtency MAtrix) is a library for testing and measuring network loss and latency between distributed endpoints.

It does this by sending UDP datagrams/probes from **collectors** to **reflectors** and measuring how long it takes for them to return, if they return at all. UDP is used to provide ECMP hashing over multiple paths (a win over ICMP) without the need for setup/teardown and per-packet granularity (a win over TCP).

## Why Is This Useful

[Black box testing](https://en.wikipedia.org/wiki/Black-box_testing) is critical to the successful monitoring and operation of a network. While collection of metrics from network devices can provide greater detail regarding known issues, they don't always provide a complete picture and can provide an overwhelming number of metrics. Black box testing with LLAMA doesn't care how the network is structured, only if it's working. This data can be used for building KPIs, observing big-picture issues, and guiding investigations into issues with unknown causes by quantifying which flows are/aren't working.

At Dropbox, we've found this useful on multiple occasions for gauging the impact of network issues on internal traffic, identifying the scope of impact, and locating issues for which we had no other metrics (internal hardware failures, circuit degradations, etc).

**Even if you operate entirely in the cloud** LLAMA can help identify reachability and network health issues between and within regions/zones.

## Architecture

- **Reflector** - Lightweight daemon for receiving probes and sending them back to their source.
- **Collector** - Sends probes to reflectors on potentially multiple ports, records results, and presents summarized data via REST API.
- **Scraper** - Pulls results from REST API on collectors and writes to database (currently InfluxDB).

## Quick Start

If you're looking to get started quickly with a basic setup that doesn't involve special integrations or customization, this should get you going. This assumes you have a running InfluxDB instance on locahost listening on port 5086 with a `llama` database already created.

In your Go development environment, in separate windows:

- `go run github.com/dropbox/llama/cmd/reflector`
- `go run github.com/dropbox/llama/cmd/collector`
- `go run github.com/dropbox/llama/cmd/scraper`

If you want to run each of these on a separate machine/instance, after distributing the binaries created with `go build`, customizing the flags as needed:

- `reflector -port <port>` to start the reflector listening on a non-default port.
- `collector -llama.dst-port <port> -llama.config <config>` where the port matches what the reflector is listening on, and the config is a YAML configuration based on one of the examples under `configs/`.
- `scraper -llama.collector-hosts <hosts> -llama.collector-port <port> -llama.influxdb-host <hostname> -llama.influxdb-name <db-name> -llama.influxdb-pass <pass> -llama.influxdb-port <port> -llama.influxdb-user <user> -llama.interval <seconds>`
    - `collector-hosts` being a comma-separated list of IP addresses or hostnames where collectors can be reached
    - `collector-port` identifying the port on which the collector's API is configured to listen
    - `influxdb-*` detailing where the InfluxDB instance can be reached, credentials, and database
    - `interval` being how often, in seconds, the scraper should pull data from collectors and write to the database. Should align with the summarization interval in the collector config.

## Ongoing Development

LLAMA was primarily built during a [Dropbox Hack Week](https://www.theverge.com/2014/7/24/5930927/why-dropbox-gives-its-employees-a-week-to-do-whatever-they-want) and is still considered unstable, as the API, config format, and overall design is not considered final. It works and we've been using the original internal version for quite a while, but we want to make various changes and improvements before considering a v1.0.0 release.

## Contributing

At this time, we're not ready for external contributors. Once we have a v1.0.0 release, we'll happily reconsider this and update accordingly. When that happens, substantial contributors will need to agree to the [Dropbox Contributor License Agreement](https://opensource.dropbox.com/cla/).

## Acknowledgements/References

* Inspired by: <https://www.youtube.com/watch?v=N0lZrJVdI9A>
    * With slides: <https://www.nanog.org/sites/default/files/Lapukhov_Move_Fast_Unbreak.pdf>
* Concepts borrowed from: <https://github.com/facebook/UdpPinger/>
* Looking for the legacy Python version?: https://github.com/dropbox/llama-archive
