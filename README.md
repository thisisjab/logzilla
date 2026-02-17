# LogZilla

**LogZilla is under active development. Features and APIs may change as we work towards a stable release.**

---

Logzilla is a lightweight, high-performance tool for collecting, processing, and analyzing logs from various sources. It provides a flexible pipeline architecture that enables real-time log ingestion, transformation, and querying.

## Architecture

LogZilla consists of four core components that work together to provide a complete log management solution:

### Log Source
A source defines where LogZilla collects logs from. The system supports multiple source types including:
- **File sources:** Tail log files with automatic rotation handling
- **Network sockets:** Collect logs from TCP/UDP endpoints (coming soon)
- **Redis:** Subscribe to Redis channels for log messages (coming soon)
- **Kafka:** Consume from Kafka topics (coming soon)

### Processor
Processors transform raw log lines into structured, queryable data. Built-in processors include:
- **JSON parser:** Extract fields from JSON-formatted logs
- **Regex extractor:** Parse custom log formats using regular expressions (coming soon)
- **Lua processor:** Write custom processing logic using Lua scripts
- **Grok patterns:** Support for common log format patterns (coming soon)

### Querier (coming soon)
The querier component provides a search interface for querying processed logs. It supports:
- Full-text search across all log fields
- Time-range based queries
- Field-specific filtering

### UI (coming soon)
A modern web-based interface that enables users to:
- Visualize log data in real-time
- Set up alerts and notifications
- Explore logs through an intuitive search interface
- Manage configurations and processing rules

## How to Install

*Installation instructions will be provided in a future release.*

## Basic Usage

### Configuration File Structure

LogZilla uses a YAML configuration file to define the entire log processing pipeline. Here's a minimal example:

```yaml
# Basic configuration for collecting and processing application logs
sources:
  - name: my-application
    type: file
    processors: ["json-extractor"]
    config:
      path: "/var/log/myapp/app.log"

processors:
  - name: json-extractor
    type: json
    config:
      timestamp_field: "timestamp"
      message_field: "message"
      level_field: "level"

storage:
  type: clickhouse
  config:
    addr: ["localhost:9000"]
    database: logs
    username: default
    password: "your secret password"

logger:
  level: info
```

### Starting LogZilla

Once you have your configuration file ready, start LogZilla with:

```bash
logzilla -config /path/to/config.yaml
```

### Common Operations

#### 1. Tail a log file in real-time
```yaml
sources:
  - name: realtime-logs
    type: file
    processors: ["json-parser"]
    config:
      path: "/var/log/application/current.log"
```

#### 2. Parse custom log formats with regex
```yaml
processors:
  - name: apache-parser
    type: regex
    config:
      pattern: '^(?P<ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) \S+" (?P<status>\d+) (?P<size>\d+)$'
      timestamp_format: "02/Jan/2006:15:04:05 -0700"
```

#### 3. Apply multiple processors to enrich logs
```yaml
sources:
  - name: enriched-logs
    type: file
    processors: ["json-parser", "geo-enricher", "anomaly-detector"]
    config:
      path: "/var/log/nginx/access.log"

processors:
  - name: json-parser
    type: json

  - name: geo-enricher
    type: lua
    config:
      script-path: "/etc/logzilla/processors/geo.lua"

  - name: anomaly-detector
    type: lua
    config:
      script-path: "/etc/logzilla/processors/anomaly.lua"
```

#### 4. Query logs using the command line

Coming soon.

### Monitoring and Management

LogZilla provides several endpoints for monitoring the system's health:

Coming soon

```bash
# Check system status
curl http://localhost:8080/health

# View processing statistics
curl http://localhost:8080/metrics

# List active sources
curl http://localhost:8080/api/v1/sources
```

### Performance Tuning

For production deployments, adjust these parameters based on your workload:

```yaml
# Increase parallelism for high-volume logs
processor_workers_count: 50

# Larger buffers for bursty traffic
raw_logs_buffer_size: 10000
processed_logs_buffer_size: 5000

# More frequent flushes for real-time requirements
storage_flush_interval: 1s
```

## How to Contribute

We welcome contributions from the community! Please read the [CONTRIBUTING.md](docs/CONTRIBUTING.md) file for detailed information about:

- Development environment setup
- Coding standards and guidelines
- Pull request process
- Testing requirements
- Code review process

### Quick Start for Contributors

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/logzilla.git`
3. Create a feature branch: `git checkout -b feature/amazing-feature`
4. Make your changes and write tests
5. Run the test suite: `make test`
6. Commit your changes: `git commit -m 'feat: an amazing feature'`
7. Push to your branch: `git push origin feature/amazing-feature`
8. Open a Pull Request

For major changes, please open an issue first to discuss what you would like to change.

### Development Resources

- **Issue Tracker:** GitHub Issues
