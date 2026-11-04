import type { Metadata } from "next";
import { Suspense } from "react";
import { listProducts } from "@/lib/catalog";
import { first } from "@/lib/url";
import { GridSkeleton, ProductGrid } from "@/components/ProductCard";

export const metadata: Metadata = { title: "Search" };

export default function SearchPage(props: PageProps<"/search">) {
  return (
    <Suspense fallback={<GridSkeleton />}>
      <Results props={props} />
    </Suspense>
  );
}

async function Results({ props }: { props: PageProps<"/search"> }) {
  const q = (first((await props.searchParams).q) ?? "").trim();

  if (!q) {
    return <p className="text-neutral-500">Type something in the search box above.</p>;
  }

  // The service returns the 50 best matches, most relevant first, with no paging.
  const page = await listProducts({ q });

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight">
        {page.items.length} result{page.items.length === 1 ? "" : "s"} for &ldquo;{q}&rdquo;
      </h1>
      <div className="mt-6">
        {page.items.length === 0 ? (
          <p className="text-neutral-500">
            Nothing found. Try a simpler word, like &ldquo;silk&rdquo; or &ldquo;saree&rdquo;.
          </p>
        ) : (
          <ProductGrid items={page.items} />
        )}
      </div>
    </div>
  );
}
