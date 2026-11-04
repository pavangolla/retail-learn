import Link from "next/link";
import { Suspense } from "react";
import { listCollections, listProducts } from "@/lib/catalog";
import { GridSkeleton, ProductGrid } from "@/components/ProductCard";

// Server component. The static shell renders immediately; each block that
// calls the Go service sits in its own <Suspense> so it streams in on its own
// (required by Next 16 "Cache Components" for live data).

async function Collections() {
  const collections = await listCollections();
  return (
    <ul className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5">
      {collections.map((c) => (
        <li key={c.handle}>
          <Link
            href={`/collections/${c.handle}`}
            className="block rounded-lg border border-neutral-200 p-4 hover:border-neutral-900"
          >
            <span className="font-medium">{c.title}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

async function BestSellers() {
  const page = await listProducts({ tag: ["best-seller"], limit: 8 });
  return <ProductGrid items={page.items} />;
}

export default function Home() {
  return (
    <div className="space-y-12">
      <section>
        <h1 className="text-3xl font-semibold tracking-tight">Shop by collection</h1>
        <div className="mt-4">
          <Suspense
            fallback={<div className="h-24 animate-pulse rounded-lg bg-neutral-100" />}
          >
            <Collections />
          </Suspense>
        </div>
      </section>

      <section>
        <h2 className="text-2xl font-semibold tracking-tight">Best sellers</h2>
        <div className="mt-4">
          <Suspense fallback={<GridSkeleton />}>
            <BestSellers />
          </Suspense>
        </div>
      </section>
    </div>
  );
}
