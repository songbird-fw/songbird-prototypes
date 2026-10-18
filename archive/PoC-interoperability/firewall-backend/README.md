# Go + XDP + nftables Firewall Backend Scaffold

Questo scaffold estende la base iniziale con:
- JWT access + refresh token
- persistenza PostgreSQL per utenti, refresh token e audit trail
- pattern ibrido XDP + nftables
- REST API Gin + SSE per la UI

## Pattern ibrido

- **XDP**: fast-path per drop immediato su blocklist IP volumetrica
- **nftables**: policy più ricche, set gestiti da userspace, casi stateful/NAT
- **PostgreSQL**: utenti, refresh token rotabili, audit log, regole persistenti
- **JWT**: access token breve + refresh token persistito e revocabile

## Flusso consigliato

1. Login → emetti access token + refresh token
2. Salva hash del refresh token in PostgreSQL
3. POST block-ip → persisti richiesta, aggiorna XDP map, aggiorna nftables set
4. Pubblica evento SSE verso la UI
5. Audit trail di ogni operazione amministrativa

## Endpoint principali

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/rules`
- `POST /api/v1/rules/block-ip`
- `DELETE /api/v1/rules/block-ip/:ip`
- `GET /api/v1/stats`
- `GET /api/v1/stream/events`

## Note

- Il refresh token va persistito come hash, non in chiaro.
- L'aggiornamento XDP e nftables va fatto nello stesso service applicativo per mantenere coerenza.
- In caso di errore su nftables, il service deve effettuare rollback logico e segnalare drift operativo.
