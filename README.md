# Seer

**Seer** is monitoring tool designed to capture autonity on-chain events, 
decode and store them in InfluxDB for analysis and visualization later on a 
UI tool.

---

## Getting Started

### Prerequisites

- **Go** (version 1.21 or higher)
- **InfluxDB** (running instance)
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

4. Run the application:
   ```bash
   cd bin
   ./seer start --config ../config/config.yaml
   ```

---

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
      - "http://35.242.168.170:8545"
   ws:
      - "wss://rpc2.piccadilly.autonity.org/ws"
   sync:
      history: true 
db:
   url: "http://127.0.0.1:8086"
   # yamllint disable-line rule:line-length
   token: "" # provide influxdb access token
   bucket: "seer_temp_1" #will be created automatically, if not exist already
   org: "autonity" #organisation needs to be created manually
abi:
   dir: "../abis"
```
---

## Usage

### CLI Commands

1. **Start the Service**:
   ```bash
   ./seer start
   ```

2. **Check Version**:
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
