import Image from "next/image";
import Script from "next/script";

export default function WattlessShowcasePage() {
  return (
    <main className="min-h-screen bg-[#08110d] px-6 py-12 text-white">
      <div className="mx-auto grid max-w-6xl gap-10 lg:grid-cols-[1fr_1.08fr] lg:items-center">
        <section className="space-y-6">
          <span className="inline-flex rounded-full border border-[rgba(155,214,126,0.22)] bg-[rgba(155,214,126,0.08)] px-3 py-1 text-xs uppercase tracking-[0.28em] text-[var(--accent-strong)]">
            Wattless Demo
          </span>
          <h1 className="max-w-3xl text-5xl font-semibold tracking-tight text-white sm:text-6xl">
            La misma historia, optimizada para cargar antes, pesar menos y desperdiciar menos.
          </h1>
          <p className="max-w-2xl text-lg leading-8 text-slate-300">
            Este before/after controlado está pensado para enseñar con claridad
            la relación entre transferencia, CO2 y Largest Contentful Paint.
          </p>
          <div className="grid gap-4 sm:grid-cols-3">
            <article className="rounded-3xl border border-[var(--line)] bg-[rgba(255,255,255,0.04)] p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Hero asset
              </div>
              <div className="mt-3 text-2xl text-white">Optimizada</div>
            </article>
            <article className="rounded-3xl border border-[var(--line)] bg-[rgba(255,255,255,0.04)] p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                JS crítico
              </div>
              <div className="mt-3 text-2xl text-white">Diferido</div>
            </article>
            <article className="rounded-3xl border border-[var(--line)] bg-[rgba(255,255,255,0.04)] p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Transferencia
              </div>
              <div className="mt-3 text-2xl text-white">Más baja</div>
            </article>
          </div>
        </section>

        <section className="space-y-6">
          <div className="overflow-hidden rounded-[2rem] border border-[var(--line)] bg-[#101813] shadow-2xl">
            <Image
              src="/showcase/hero-wattless.svg"
              alt="Hero optimizada para la demo wattless"
              width={1440}
              height={1080}
              priority
              sizes="(max-width: 1024px) 100vw, 48vw"
              className="h-auto w-full"
            />
          </div>
          <Script src="/showcase/wattless-idle.js" strategy="lazyOnload" />
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="rounded-[1.8rem] border border-[var(--line)] bg-[rgba(255,255,255,0.04)] p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Resultado esperado
              </div>
              <p className="mt-3 text-base leading-7 text-slate-200">
                Menos bytes, menor LCP y una narrativa más sólida para explicar
                por qué optimizar también es ser más sostenible.
              </p>
            </div>
            <div className="rounded-[1.8rem] border border-[var(--line)] bg-[rgba(255,255,255,0.04)] p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Qué debería detectar
              </div>
              <p className="mt-3 text-base leading-7 text-slate-200">
                Wattless debe reflejar menos transferencia, mejor score y una
                hero más eficiente en el render crítico.
              </p>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
