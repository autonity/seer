# Seer

**Seer** is monitoring tool designed to capture autonity on-chain events, 
It decodes and stores them in InfluxDB, Events are visualized later on Grafana.

---

## Getting Started

### Prerequisites

- **Go** (version 1.21 or higher)
- RPC url to an Autonity node

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/autonity/seer.git
   cd seer
   ```

2. Install dependencies:
   ```bash
   make install
   ```

3. Build the application:
   ```bash
   make build
   ```
4. Run application in default mode:
   ```bash
   make run
   ```
---

## Visualize
Go over to ```localhost:3000``` to visualize the data captured by Seer.


## Configuration

Seer uses **Viper** for configuration management. You can specify options via:
- A `config.yaml` file.
- Environment variables.
- Command-line flags.

### Sample `config.yaml`:
```yaml
seer:
   logLevel: "info"
node:
   rpc:
      maxConnections: 25  # maximum rpc connections across all urls
      urls:
         - "https://rpc1.mainnet.autonity.org"
   ws:
      maxConnections: 5 # maximum websocket connections across all urls
      urls:
         - "wss://rpc1.mainnet.autonity.org/ws"
   sync:
      history: true 
db:
   url: "http://127.0.0.1:8086"
   token: ""
   bucket: "mainnet"
   org: "autonity"
abi:
   dir: "../abis"
```
---

## Usage

### CLI Commands

1. **Start service in default mode which syncs live and historical blocks**:
   ```bash
   ./seer start --config ../config/config.yaml
   ```

2. **Start service in pull mode where only given range of blocks are synced**:
   ```bash
   ./seer pull --start-block=1000 --end-block=20000 --config ../config/config.yaml
   ```

3. **Check Version**:
   ```bash
   ./seer version
   ```

## Development

### Running Tests

Run tests for all modules:
```bash
make test
```

### Linting and Formatting

Format and lint the codebase:
```bash
make fmt
make lint
```

---

## Contributing

Contributions are welcome! Please submit a pull request or open an issue if you find a bug or have a feature request.

---

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.

---
