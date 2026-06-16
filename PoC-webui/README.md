# PoC WebUI

This is a proof of concept for a firewall WebUI.

## Features
- **Dashboard**: Real-time traffic analysis and threat assessment graphs.
- **Firewall Policies**: Management interface for network security rules.
- **Users**: User management and access control.
- **Single Binary**: The React frontend is embedded into the Go backend for easy deployment.

## Tech Stack
- **Frontend**: React, Tailwind CSS, Chart.js, Lucide React, React Router.
- **Backend**: Go (Echo framework).

## How to Build

### 1. Build the Frontend
```bash
cd frontend
npm install
npm run build
```

### 2. Build the Go Backend
```bash
cd ..
go build -o webui main.go
```

## How to Run
```bash
./webui
```
The server will start on `http://localhost:8080`.
