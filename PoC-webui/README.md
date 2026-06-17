# PoC Firewall WebUI

This is a proof-of-concept web interface for a firewall implementation.

## Requirements
- Go 1.23+
- Node.js & npm

## Structure
- `frontend/`: React + Tailwind CSS frontend application.
- `main.go`: Go backend using Echo framework, serves the embedded frontend assets.

## How to build and run

1. **Build the frontend:**
   ```bash
   cd frontend
   npm install
   npm run build
   cd ..
   ```

2. **Run the application:**
   ```bash
   go run main.go
   ```
   The application will be available at `http://localhost:8080`.

3. **Build the single binary:**
   ```bash
   CGO_ENABLED=0 go build -o firewall-webui main.go
   ```

## Features
- **Dashboard:** OPNsense-style widgets for system info, throughput (Chart.js), services, and live logs.
- **Firewall Policies:** Fortigate-style collapsible policy groups with detailed traffic inspection rules.
- **Policy Objects:** Manage Addresses, Virtual IPs, and Services.
- **User Management:** Administrator account management.
