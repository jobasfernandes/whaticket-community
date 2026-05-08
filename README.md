[![Donate](https://img.shields.io/badge/Donate-PayPal-green.svg)](https://www.paypal.com/donate?business=VWW3BHW4AWHUY&item_name=Desenvolvimento+de+Software&currency_code=BRL)
[![FOSSA Status](https://app.fossa.com/api/projects/custom%2B21084%2Fgithub.com%2Fcanove%2Fwhaticket.svg?type=shield)](https://app.fossa.com/projects/custom%2B21084%2Fgithub.com%2Fcanove%2Fwhaticket?ref=badge_shield)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=canove_whaticket&metric=alert_status)](https://sonarcloud.io/dashboard?id=canove_whaticket)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=canove_whaticket&metric=sqale_rating)](https://sonarcloud.io/dashboard?id=canove_whaticket)
[![Discord Chat](https://img.shields.io/discord/784109818247774249.svg?logo=discord)](https://discord.gg/Dp2tTZRYHg)
[![Forum](https://img.shields.io/badge/forum-online-blue.svg?logo=discourse)](https://whaticket.online/)

# WhaTicket!

**NOTE**: This fork ships a fully rewritten **Go** stack. The legacy Node.js backend (whatsapp-web.js + Baileys) has been replaced by a **Go API** and a decoupled **Go worker** that talks to WhatsApp through [whatsmeow](https://github.com/tulir/whatsmeow). The two services communicate via RabbitMQ.

A _very simple_ Ticket System based on WhatsApp messages.

Backend is a Go REST + WebSocket API that creates tickets from inbound WhatsApp messages and stores them in PostgreSQL. A separate Go worker runs the whatsmeow client (sessions, sending, media) and exchanges jobs/events with the backend over RabbitMQ. Media is stored in any S3-compatible bucket (MinIO by default).

Frontend is a full-featured multi-user _chat app_ built with React + Material UI, bundled with **Vite**, that talks to the backend over REST and WebSockets. It allows you to interact with contacts, tickets, send and receive WhatsApp messages.

**NOTE**: I can't guarantee you will not be blocked by using this method, although it has worked for me. WhatsApp does not allow bots or unofficial clients on their platform, so this shouldn't be considered totally safe.

## How it works?

On every new message received in an associated WhatsApp, a new Ticket is created. Then, this ticket can be reached in a _queue_ on _Tickets_ page, where you can assign ticket to your yourself by _aceppting_ it, respond ticket message and eventually _resolve_ it.

Subsequent messages from same contact will be related to first **open/pending** ticket found.

If a contact sent a new message in less than 2 hours interval, and there is no ticket from this contact with **pending/open** status, the newest **closed** ticket will be reopen, instead of creating a new one.

## Screenshots

![](https://github.com/canove/whaticket/raw/master/images/whaticket-queues.gif)
<img src="https://raw.githubusercontent.com/canove/whaticket/master/images/chat2.png" width="350"> <img src="https://raw.githubusercontent.com/canove/whaticket/master/images/chat3.png" width="350"> <img src="https://raw.githubusercontent.com/canove/whaticket/master/images/multiple-whatsapps2.png" width="350"> <img src="https://raw.githubusercontent.com/canove/whaticket/master/images/contacts1.png" width="350">

## Features

- Have multiple users chating in same WhatsApp Number ✅
- Connect to multiple WhatsApp accounts and receive all messages in one place ✅ 🆕
- Create and chat with new contacts without touching cellphone ✅
- Send and receive message ✅
- Send media (images/audio/documents) ✅
- Receive media (images/audio/video/documents) ✅

## Project layout

```
backend/    # Go REST + WebSocket API (chi, GORM, Postgres)
worker/     # Go whatsmeow worker (sessions, send, media, ack)
frontend/   # React + Vite + Material UI
docker-compose.local.yaml   # development stack
docker-compose.prod.yaml    # production stack (Traefik + Let's Encrypt)
.env.example                # production environment template
```

The backend talks to **PostgreSQL** for state, **RabbitMQ** for jobs/events to the worker, and **S3-compatible storage** (MinIO in dev) for media. The worker keeps its whatsmeow `sqlstore` on a local volume.

## Installation and Usage (Development)

Requirements: Docker + Docker Compose v2. (For running services natively: Go 1.22+, Node 18+, Postgres 15, RabbitMQ 3.13, MinIO.)

Clone this repo:

```bash
git clone https://github.com/canove/whaticket/ whaticket
cd whaticket
```

Bring up Postgres, RabbitMQ, MinIO, the Go backend and the Go worker:

```bash
docker compose -f docker-compose.local.yaml up -d --build
```

The dev compose file auto-runs migrations (`AUTO_MIGRATE=true`) and seeds a default admin user (`AUTO_SEED=true`). Useful endpoints:

- Backend API + WebSocket: http://localhost:8080
- Worker health: http://localhost:8081
- RabbitMQ management UI: http://localhost:15672 (`whaticket` / `whaticket`)
- MinIO console: http://localhost:9001 (`minioadmin` / `minioadmin`)
- MinIO S3 endpoint: http://localhost:9000

Run the frontend. Either start the Vite dev server natively (recommended for hot reload):

```bash
cd frontend
cp .env.example .env   # set VITE_BACKEND_URL=http://localhost:8080/
npm install
npm run dev            # serves on http://localhost:3000
```

…or build and run the frontend container that ships with the compose file:

```bash
docker compose -f docker-compose.local.yaml --profile frontend up -d --build frontend
```

Available local environment overrides (all have sane defaults — see `docker-compose.local.yaml`):

```bash
JWT_SECRET                 # default: dev placeholder; override for non-trivial work
JWT_REFRESH_SECRET         # default: dev placeholder
SEED_ADMIN_EMAIL           # default: admin@whaticket.com
SEED_ADMIN_PASSWORD        # default: admin
AUTO_MIGRATE               # default: true
AUTO_SEED                  # default: true
FRONTEND_URL               # default: http://localhost:3000
VITE_BACKEND_URL           # default: http://localhost:8080/
```

Running the Go services natively (without Docker) is also supported:

```bash
# backend
cd backend
go run ./cmd/api          # reads env vars listed in docker-compose.local.yaml

# worker
cd worker
go run ./cmd/worker
```

Use the app:

- Go to http://localhost:3000/signup (or `/login` with the seeded admin: `admin@whaticket.com` / `admin`).
- On the sidebar, go to _Connections_ and create your first WhatsApp connection.
- Wait for the QR CODE button to appear, click it and scan with your phone.
- Done. Every message received by your synced WhatsApp number will appear in the Tickets list.

## Production deployment

The production stack is `docker-compose.prod.yaml`. It runs **Traefik** as the reverse proxy (with automatic Let's Encrypt certificates), Postgres, RabbitMQ, MinIO, the backend, the worker and the frontend. There is **no Nginx, no PM2, no Puppeteer**.

You'll need a host with Docker + Docker Compose v2 and DNS records pointing at it for three subdomains. Defaults in the example below: `app.example.com` (frontend), `api.example.com` (backend), `storage.example.com` (MinIO).

Clone the repo and create your `.env` from the template:

```bash
git clone https://github.com/canove/whaticket whaticket
cd whaticket
cp .env.example .env
```

Fill `.env` — every variable matters. The full template is in `.env.example`; the key ones:

```bash
# Image tags (see "Docker images" below)
BACKEND_TAG=latest
WORKER_TAG=latest
FRONTEND_TAG=latest

# Public hostnames (DNS A/AAAA records → Traefik host)
BACKEND_HOST=api.example.com
FRONTEND_HOST=app.example.com
MINIO_HOST=storage.example.com

# Let's Encrypt
ACME_EMAIL=ops@example.com

# Postgres
POSTGRES_USER=whaticket
POSTGRES_PASSWORD=change-me
POSTGRES_DB=whaticket

# RabbitMQ
RABBITMQ_USER=whaticket
RABBITMQ_PASSWORD=change-me

# MinIO (S3-compatible)
MINIO_ROOT_USER=change-me
MINIO_ROOT_PASSWORD=change-me-min-8-chars
MINIO_BUCKET=whaticket-media
MINIO_PUBLIC_URL=https://storage.example.com

# JWT (32+ random bytes each, different secrets)
JWT_SECRET=replace-with-32-byte-random-secret
JWT_REFRESH_SECRET=replace-with-different-32-byte-random-secret

# Frontend wiring
FRONTEND_URL=https://app.example.com
FRONTEND_BACKEND_URL=https://api.example.com

# Logging
LOG_LEVEL=info
```

Bring the stack up:

```bash
docker compose -f docker-compose.prod.yaml up -d
```

The compose file ships a one-shot `migrate` service that runs `migrate up` against Postgres before the backend starts, so the database is ready on the first boot. Traefik handles HTTPS via Let's Encrypt (TLS-ALPN-01 challenge on port 443) — make sure ports 80 and 443 are reachable on the host.

Stream logs and stop:

```bash
docker compose -f docker-compose.prod.yaml logs -f backend worker
docker compose -f docker-compose.prod.yaml down
```

## Access Data

User: admin@whaticket.com
Password: admin

## Upgrading

WhaTicket is a work in progress and new features land frequently. To update an existing production deployment, bump the image tags in `.env`, then pull and recreate.

**Note**: Always check `.env.example` and adjust your `.env` file before upgrading, since new variables may have been added.

```bash
cd ~/whaticket
git pull

# (optional) edit .env to point at a new tag, e.g. BACKEND_TAG=1.5.0
nano .env

docker compose -f docker-compose.prod.yaml pull
docker compose -f docker-compose.prod.yaml up -d
```

The one-shot `migrate` service runs every time the stack starts and applies any pending database migrations before the backend comes up.

## Contributing

This project helps you and you want to help keep it going? Buy me a coffee:

<a href="https://www.buymeacoffee.com/canove" target="_blank"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: 61px !important;width: 174px !important;box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;" ></a>

Para doações em BRL, utilize o Paypal:

[![Donate](https://img.shields.io/badge/Donate-PayPal-green.svg)](https://www.paypal.com/donate?business=VWW3BHW4AWHUY&item_name=Desenvolvimento+de+Software&currency_code=BRL)

Any help and suggestions will be apreciated.

## Disclaimer

I just started leaning Javascript a few months ago and this is my first project. It may have security issues and many bugs. I recommend using it only on local network.

This project is not affiliated, associated, authorized, endorsed by, or in any way officially connected with WhatsApp or any of its subsidiaries or its affiliates. The official WhatsApp website can be found at https://whatsapp.com. "WhatsApp" as well as related names, marks, emblems and images are registered trademarks of their respective owners.
