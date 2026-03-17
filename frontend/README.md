# Goster IoT Frontend

Next.js 16 + React 19 dashboard for Goster IoT device management.

## Requirements

- Node.js 20+
- pnpm 10+

## Quick Start

```bash
cd frontend
pnpm install
pnpm dev
```

Open `http://localhost:3000`.

## Environment Variables

Create `frontend/.env.local`:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
```

`NEXT_PUBLIC_API_URL` can include `/api/v1` or not. The client and proxy layer will normalize it.

## Available Scripts

- `pnpm dev`: start local dev server
- `pnpm build`: production build
- `pnpm start`: start production server
- `pnpm lint`: run ESLint
- `pnpm test --run`: run Vitest once
- `pnpm test:coverage`: run coverage report
- `pnpm gen-types`: regenerate OpenAPI types from `../contracts/openapi.yaml`

## Architecture Notes

- App Router layout in `src/app`.
- Route protection uses `src/proxy.ts`.
- Dashboard auth guard is server-side in `src/app/(dashboard)/layout.tsx`.
- API calls are centralized in `src/lib/api-client.ts`.
- API types are generated in `src/lib/api-types.ts`.
- UI building blocks live in `src/components/ui`.

## Permission Model

- `0`: waiting for approval
- `1`: read-only
- `2`: read-write
- `3`: admin

## Testing and CI

- Local checks:

```bash
pnpm lint
pnpm test --run
pnpm build
```

- GitHub Actions workflow: `.github/workflows/frontend-ci.yml`
