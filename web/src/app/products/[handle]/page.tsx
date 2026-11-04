import { notFound } from "next/navigation";
import { Suspense } from "react";
import { getProduct } from "@/lib/catalog";
import { humanize } from "@/lib/format";
import { VariantPicker } from "./VariantPicker";

export default function ProductPage(props: PageProps<"/products/[handle]">) {
  return (
    <Suspense fallback={<div className="h-96 animate-pulse rounded-lg bg-neutral-100" />}>
      <ProductView props={props} />
    </Suspense>
  );
}

async function ProductView({ props }: { props: PageProps<"/products/[handle]"> }) {
  const { handle } = await props.params;
  const product = await getProduct(handle);
  if (!product) notFound();

  const image = product.images[0];

  return (
    <div className="grid gap-8 md:grid-cols-2">
      <div className="aspect-[4/5] overflow-hidden rounded-lg bg-neutral-100">
        {image && (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={image.url} alt={image.alt} className="h-full w-full object-cover" />
        )}
      </div>

      <div>
        <p className="text-sm text-neutral-500">{product.productType}</p>
        <h1 className="mt-1 text-3xl font-semibold tracking-tight">{product.title}</h1>

        {/* The one interactive piece: a client component. The server sends it the
            product data; it picks a variant and shows that variant's price. */}
        <VariantPicker options={product.options} variants={product.variants} />

        <p className="mt-6 text-neutral-700">{product.description}</p>

        <ul className="mt-4 flex flex-wrap gap-2">
          {product.tags.map((t) => (
            <li key={t} className="rounded-full bg-neutral-100 px-3 py-1 text-xs">
              {humanize(t)}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
