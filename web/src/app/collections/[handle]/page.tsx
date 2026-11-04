import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import {
  getCollection,
  getCollectionFacets,
  listCollectionProducts,
  type ListParams,
} from "@/lib/catalog";
import { formatMoney } from "@/lib/format";
import { first, href, toInt, type SearchParams } from "@/lib/url";
import { GridSkeleton, ProductGrid } from "@/components/ProductCard";

// Price presets, in cents. `max` is inclusive in the API.
const PRICE_RANGES = [
  { label: "Under $30", min: undefined, max: 3000 },
  { label: "$30 to $60", min: 3000, max: 6000 },
  { label: "$60 to $150", min: 6000, max: 15000 },
  { label: "Over $150", min: 15000, max: undefined },
];

const SORTS = [
  { value: undefined, label: "Featured" },
  { value: "price_asc", label: "Price: low to high" },
  { value: "price_desc", label: "Price: high to low" },
  { value: "newest", label: "Newest" },
];

// In Next 16, `params` and `searchParams` are Promises. Reading them is dynamic,
// so the page itself stays a plain synchronous shell and the inner component
// that awaits them sits inside <Suspense>.
export default function CollectionPage(props: PageProps<"/collections/[handle]">) {
  return (
    <Suspense fallback={<GridSkeleton />}>
      <CollectionView props={props} />
    </Suspense>
  );
}

async function CollectionView({
  props,
}: {
  props: PageProps<"/collections/[handle]">;
}) {
  const { handle } = await props.params;
  const sp: SearchParams = await props.searchParams;

  const params: ListParams = {
    sort: first(sp.sort),
    type: first(sp.type),
    minPrice: toInt(first(sp.min)),
    maxPrice: toInt(first(sp.max)),
    after: first(sp.after),
    limit: 12,
  };
  // Facets describe the filters, so they ignore sort and paging.
  const facetParams: ListParams = {
    minPrice: params.minPrice,
    maxPrice: params.maxPrice,
  };
  const typeFacetParams: ListParams = { ...facetParams, type: undefined };

  // Three independent calls: start them together instead of one after another.
  const [collection, page, facets] = await Promise.all([
    getCollection(handle),
    listCollectionProducts(handle, params),
    getCollectionFacets(handle, typeFacetParams),
  ]);
  if (!collection || !page) notFound();

  const path = `/collections/${handle}`;
  const link = (c: Record<string, string | undefined>) => href(path, sp, c);
  const activeLink = "font-semibold underline";

  return (
    <div>
      <h1 className="text-3xl font-semibold tracking-tight">{collection.title}</h1>
      <p className="mt-1 text-neutral-500">{collection.description}</p>

      <div className="mt-8 grid gap-8 md:grid-cols-[14rem_1fr]">
        <aside className="space-y-6 text-sm">
          <div>
            <h2 className="mb-2 font-semibold">Sort</h2>
            <ul className="space-y-1">
              {SORTS.map((s) => (
                <li key={s.label}>
                  <Link
                    href={link({ sort: s.value })}
                    className={first(sp.sort) === s.value ? activeLink : "hover:underline"}
                  >
                    {s.label}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          {facets?.productTypes && facets.productTypes.length > 0 && (
            <div>
              <h2 className="mb-2 font-semibold">Type</h2>
              <ul className="space-y-1">
                <li>
                  <Link
                    href={link({ type: undefined })}
                    className={!params.type ? activeLink : "hover:underline"}
                  >
                    All
                  </Link>
                </li>
                {facets.productTypes.map((t) => (
                  <li key={t.value}>
                    <Link
                      href={link({ type: t.value })}
                      className={params.type === t.value ? activeLink : "hover:underline"}
                    >
                      {t.value} ({t.count})
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div>
            <h2 className="mb-2 font-semibold">Price</h2>
            <ul className="space-y-1">
              <li>
                <Link
                  href={link({ min: undefined, max: undefined })}
                  className={
                    !params.minPrice && !params.maxPrice ? activeLink : "hover:underline"
                  }
                >
                  Any price
                </Link>
              </li>
              {PRICE_RANGES.map((r) => (
                <li key={r.label}>
                  <Link
                    href={link({
                      min: r.min?.toString(),
                      max: r.max?.toString(),
                    })}
                    className={
                      params.minPrice === r.min && params.maxPrice === r.max
                        ? activeLink
                        : "hover:underline"
                    }
                  >
                    {r.label}
                  </Link>
                </li>
              ))}
            </ul>
            {facets && facets.total > 0 && (
              <p className="mt-2 text-xs text-neutral-500">
                Prices here run {formatMoney(facets.priceMinCents)} to{" "}
                {formatMoney(facets.priceMaxCents)}.
              </p>
            )}
          </div>
        </aside>

        <section>
          {page.items.length === 0 ? (
            <p className="text-neutral-500">No products match these filters.</p>
          ) : (
            <ProductGrid items={page.items} />
          )}

          <nav className="mt-10 flex gap-6 text-sm">
            {params.after && (
              <Link href={link({ after: undefined })} className="underline">
                Back to first page
              </Link>
            )}
            {page.nextCursor && (
              <Link href={link({ after: page.nextCursor })} className="underline">
                Next page
              </Link>
            )}
          </nav>
        </section>
      </div>
    </div>
  );
}
