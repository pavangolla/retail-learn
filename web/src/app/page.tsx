import { Suspense } from "react";

// This is a server component: it runs on the server, so the browser never
// talks to the Go service directly. That means no CORS setup and the
// backend URL stays private.
//
// Next.js 16 "Cache Components": pages are dynamic by default, and anything
// that fetches live data must sit inside a <Suspense> boundary. The page
// shell below renders instantly; <CatalogStatus /> streams in when the
// fetch to the Go service finishes.

type Health = { status: string; service: string; mongo: string };

type CatalogResult =
  | { ok: true; data: Health }
  | { ok: false; data?: Health; error?: string };

async function getCatalogHealth(): Promise<CatalogResult> {
  const base = process.env.CATALOG_URL ?? "http://localhost:8081";
  try {
    const res = await fetch(`${base}/health`, {
      cache: "no-store",
      signal: AbortSignal.timeout(3000),
    });
    const data = (await res.json()) as Health;
    return res.ok ? { ok: true, data } : { ok: false, data };
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : "unreachable",
    };
  }
}

async function CatalogStatus() {
  const result = await getCatalogHealth();

  return (
    <section
      className={`rounded-lg border p-5 ${
        result.ok
          ? "border-green-300 bg-green-50 text-green-900"
          : "border-red-300 bg-red-50 text-red-900"
      }`}
    >
      <div className="flex items-center gap-2 font-medium">
        <span
          className={`h-2.5 w-2.5 rounded-full ${
            result.ok ? "bg-green-500" : "bg-red-500"
          }`}
        />
        Catalog service: {result.ok ? "healthy" : "not healthy"}
      </div>

      {result.data ? (
        <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="opacity-70">status</dt>
          <dd>{result.data.status}</dd>
          <dt className="opacity-70">service</dt>
          <dd>{result.data.service}</dd>
          <dt className="opacity-70">mongo</dt>
          <dd>{result.data.mongo}</dd>
        </dl>
      ) : (
        <p className="mt-3 text-sm">
          Could not reach the catalog service
          {result.ok ? "" : result.error ? `: ${result.error}` : "."}
        </p>
      )}
    </section>
  );
}

function StatusSkeleton() {
  return (
    <section className="rounded-lg border border-neutral-300 bg-neutral-50 p-5 text-neutral-600">
      Checking catalog service...
    </section>
  );
}

export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-6 p-8">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight">Retail-Learn</h1>
        <p className="mt-1 text-sm text-neutral-500">
          Phase 0: is the storefront wired to the backend?
        </p>
      </header>

      <Suspense fallback={<StatusSkeleton />}>
        <CatalogStatus />
      </Suspense>

      <p className="text-xs text-neutral-500">
        Start the backend with <code>cd services/catalog &amp;&amp; go run .</code>
        , then refresh.
      </p>
    </main>
  );
}
