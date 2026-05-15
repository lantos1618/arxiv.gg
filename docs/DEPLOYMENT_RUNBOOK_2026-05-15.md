# arxiv.gg Production Hardening Runbook

Date: 2026-05-15

## Goals

- Preserve the existing `arxiv_postgres_data` Docker volume. This volume contains the paper cache and embeddings.
- Avoid destructive database operations. Do not run `docker compose down -v`, `docker volume rm`, `DROP`, `TRUNCATE`, or schema rewrites.
- Keep downtime close to zero by preparing the image and environment before swapping the app container.

## Preflight

1. Confirm the live database volume exists:

   ```sh
   docker volume inspect arxiv_postgres_data
   ```

2. Confirm Compose will use the external volume, not create a fresh one:

   ```sh
   docker compose config | grep -A4 arxiv_postgres_data
   ```

3. Create or update `.env` on the box from `.env.example`:

   ```sh
   cp -n .env.example .env
   ```

   Replace every placeholder with production values. Use the current Postgres password from the live container if keeping the existing DB user. Keep `DATABASE_URL` pointed at `arxiv-postgres` for the first app-only Compose adoption.

4. Build before touching the running app:

   ```sh
   docker compose build arxiv
   ```

5. Check the database directly:

   ```sh
   docker exec arxiv-postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"
   ```

## Database Index Mitigation

The author-search mitigation is non-destructive but should still be run intentionally:

```sh
set -a
. ./.env
set +a
docker compose exec -T postgres psql -U "${POSTGRES_USER:-arxiv}" -d "${POSTGRES_DB:-arxiv}" < deploy/sql/2026-05-15-author-trigram-index.sql
```

Notes:

- `CREATE INDEX CONCURRENTLY` avoids table-wide write blocking.
- Do not wrap this script in `BEGIN`.
- If it is interrupted, check for invalid indexes before rerunning.

## App Swap

For the first Compose adoption, keep the existing Postgres container running and replace only the app container:

```sh
docker stop arxiv-container
docker rm arxiv-container
docker compose up -d --no-deps arxiv
```

This keeps Postgres running, joins the existing external `arxiv-network`, and preserves `arxiv_postgres_data`. Expected downtime is the few seconds between stopping the old app container and the new app passing `/health`.

## Verification

```sh
curl -fsS http://127.0.0.1/health
docker compose ps
docker logs --since 5m arxiv-container
docker logs --since 5m cloudflared
```

Expected health response:

```json
{"success":true,"data":{"db":"postgres","status":"ok"}}
```

## Rollback

If the new app is unhealthy, leave Postgres alone and roll back only the app image/container:

```sh
docker stop arxiv-container
docker rm arxiv-container
docker compose up -d --no-deps arxiv
```

Use the previous image tag or previous Git commit if the image was tagged before deploy.

## Never Do During This Deploy

- `docker compose down -v`
- `docker volume rm arxiv_postgres_data`
- `docker system prune --volumes`
- Recreate Postgres with a different anonymous volume
- Put tunnel tokens, DB passwords, or admin tokens in tracked files
