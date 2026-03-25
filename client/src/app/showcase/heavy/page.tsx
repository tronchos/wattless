/* eslint-disable @next/next/no-img-element */

import Script from "next/script";

export default function HeavyShowcasePage() {
  return (
    <main className="min-h-screen bg-[#090d0b] px-6 py-12 text-white">
      <Script src="/showcase/heavy-blocker.js" strategy="beforeInteractive" />
      <Script src="/showcase/heavy-vendor.js" strategy="beforeInteractive" />
      <div className="mx-auto grid max-w-6xl gap-10 lg:grid-cols-[1fr_1.08fr] lg:items-center">
        <section className="space-y-6">
          <span className="inline-flex rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs uppercase tracking-[0.28em] text-amber-200">
            Heavy Demo
          </span>
          <h1 className="max-w-3xl text-5xl font-semibold tracking-tight text-white sm:text-6xl">
            Una landing atractiva que paga un precio demasiado alto en bytes.
          </h1>
          <p className="max-w-2xl text-lg leading-8 text-slate-300">
            Esta versión exagera lo que Wattless debe detectar: hero pesada,
            script bloqueante, tipografía ruidosa y más coste del necesario en
            el render inicial.
          </p>
          <div className="grid gap-4 sm:grid-cols-3">
            <article className="rounded-3xl border border-white/10 bg-white/5 p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Hero asset
              </div>
              <div className="mt-3 text-2xl text-white">Pesada</div>
            </article>
            <article className="rounded-3xl border border-white/10 bg-white/5 p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                JS crítico
              </div>
              <div className="mt-3 text-2xl text-white">Pesado + bloqueante</div>
            </article>
            <article className="rounded-3xl border border-white/10 bg-white/5 p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Terceros
              </div>
              <div className="mt-3 text-2xl text-white">Simulados</div>
            </article>
          </div>
        </section>

        <section className="space-y-6">
          <img
            src="/showcase/heavy-hero"
            alt="Hero pesada para la demo heavy"
            className="w-full rounded-[2rem] border border-white/10 bg-[#111713] shadow-2xl"
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="rounded-[1.8rem] border border-white/10 bg-white/5 p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Storytelling
              </div>
              <p className="mt-3 text-base leading-7 text-slate-200">
                Diseño llamativo, pero con demasiada transferencia en el primer
                golpe de red.
              </p>
            </div>
            <div className="rounded-[1.8rem] border border-white/10 bg-white/5 p-5">
              <div className="text-sm uppercase tracking-[0.24em] text-slate-400">
                Qué debería pasar
              </div>
              <p className="mt-3 text-base leading-7 text-slate-200">
                Wattless debe marcar un LCP peor, más CO2 y una oportunidad de
                ahorro obvia en la hero.
              </p>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
