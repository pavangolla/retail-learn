import Link from "next/link";

export default function NotFound() {
  return (
    <div className="py-20 text-center">
      <h1 className="text-3xl font-semibold">Page not found</h1>
      <p className="mt-2 text-neutral-500">That product or collection does not exist.</p>
      <Link href="/" className="mt-6 inline-block underline">
        Back to the store
      </Link>
    </div>
  );
}
