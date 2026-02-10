export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-5xl flex-col gap-8 px-6 py-16 sm:px-10 lg:px-12">
      <section className="space-y-3">
        <p className="text-sm font-medium uppercase tracking-wide text-accent">Memora</p>
        <h1 className="text-4xl font-semibold leading-tight text-textPrimary sm:text-5xl">
          Next.js 14 App Router starter configured with TypeScript and Tailwind CSS.
        </h1>
        <p className="max-w-2xl text-base text-textSecondary sm:text-lg">
          This baseline project includes smooth scrolling, global typography defaults, custom color tokens,
          and GSAP ready for animations.
        </p>
      </section>

      <section className="grid gap-4 sm:grid-cols-2">
        <article className="rounded-2xl bg-cardSky p-6">
          <h2 className="text-lg font-semibold text-textPrimary">Fast setup</h2>
          <p className="mt-2 text-textSecondary">App Router structure and strict TypeScript defaults are in place.</p>
        </article>
        <article className="rounded-2xl bg-cardSun p-6">
          <h2 className="text-lg font-semibold text-textPrimary">Tailwind ready</h2>
          <p className="mt-2 text-textSecondary">Content paths scan the app and components directories.</p>
        </article>
        <article className="rounded-2xl bg-cardMint p-6">
          <h2 className="text-lg font-semibold text-textPrimary">Design tokens</h2>
          <p className="mt-2 text-textSecondary">Core semantic colors are defined as CSS variables and Tailwind colors.</p>
        </article>
        <article className="rounded-2xl bg-cardLavender p-6">
          <h2 className="text-lg font-semibold text-textPrimary">Animation support</h2>
          <p className="mt-2 text-textSecondary">GSAP is installed for motion-rich interactions.</p>
        </article>
      </section>
    </main>
  );
}
