import Link from "next/link";
import type { Summary } from "@/lib/catalog";
import { formatMoney } from "@/lib/format";

export function ProductCard({ product }: { product: Summary }) {
  const onSale =
    product.compareAtFromCents !== undefined &&
    product.compareAtFromCents > product.priceFromCents;

  return (
    <Link href={`/products/${product.handle}`} className="group block">
      <div className="relative aspect-[4/5] overflow-hidden rounded-lg bg-neutral-100">
        {product.image && (
          // A plain <img> on purpose: these are small local SVG placeholders, and
          // next/image refuses SVG unless you opt in. Swap to next/image when we
          // have real photos.
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={product.image.url}
            alt={product.image.alt}
            className="h-full w-full object-cover transition group-hover:scale-105"
          />
        )}
        {onSale && (
          <span className="absolute left-2 top-2 rounded bg-red-600 px-2 py-0.5 text-xs font-medium text-white">
            Sale
          </span>
        )}
      </div>
      <h3 className="mt-2 line-clamp-2 text-sm font-medium group-hover:underline">
        {product.title}
      </h3>
      <p className="text-sm">
        <span className="text-neutral-500">From </span>
        <span className="font-semibold">
          {formatMoney(product.priceFromCents, product.currency)}
        </span>
        {onSale && (
          <span className="ml-2 text-neutral-400 line-through">
            {formatMoney(product.compareAtFromCents!, product.currency)}
          </span>
        )}
      </p>
    </Link>
  );
}

export function ProductGrid({ items }: { items: Summary[] }) {
  return (
    <ul className="grid grid-cols-2 gap-x-4 gap-y-8 md:grid-cols-3 lg:grid-cols-4">
      {items.map((p) => (
        <li key={p.handle}>
          <ProductCard product={p} />
        </li>
      ))}
    </ul>
  );
}

export function GridSkeleton({ count = 8 }: { count?: number }) {
  return (
    <ul className="grid grid-cols-2 gap-x-4 gap-y-8 md:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: count }, (_, i) => (
        <li key={i}>
          <div className="aspect-[4/5] animate-pulse rounded-lg bg-neutral-100" />
          <div className="mt-2 h-4 w-3/4 animate-pulse rounded bg-neutral-100" />
          <div className="mt-1 h-4 w-1/3 animate-pulse rounded bg-neutral-100" />
        </li>
      ))}
    </ul>
  );
}
