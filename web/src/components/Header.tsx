import Link from "next/link";

// The main navigation is a short hand-picked list of collection handles. The
// home page lists every collection straight from the API.
const NAV = [
  { href: "/collections/women", label: "Women" },
  { href: "/collections/men", label: "Men" },
  { href: "/collections/girls", label: "Girls" },
  { href: "/collections/sarees", label: "Sarees" },
  { href: "/collections/footwear", label: "Footwear" },
  { href: "/collections/sale", label: "Sale" },
];

export function Header() {
  return (
    <header className="border-b border-neutral-200">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-8 gap-y-3 px-4 py-4">
        <Link href="/" className="text-xl font-semibold tracking-tight">
          Retail-Learn
        </Link>
        <nav className="order-last flex w-full gap-5 overflow-x-auto text-sm md:order-none md:w-auto">
          {NAV.map((n) => (
            <Link key={n.href} href={n.href} className="hover:underline">
              {n.label}
            </Link>
          ))}
        </nav>
        {/* A plain GET form: submitting it just navigates to /search?q=... */}
        <form action="/search" className="ml-auto flex">
          <input
            type="search"
            name="q"
            placeholder="Search products"
            aria-label="Search products"
            className="w-44 rounded-l border border-neutral-300 px-3 py-1.5 text-sm md:w-64"
          />
          <button
            type="submit"
            className="rounded-r border border-l-0 border-neutral-300 bg-neutral-900 px-3 py-1.5 text-sm text-white"
          >
            Search
          </button>
        </form>
      </div>
    </header>
  );
}
