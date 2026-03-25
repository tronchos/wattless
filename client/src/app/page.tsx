import { ScanWorkbench } from "@/components/scan-workbench";

export default function HomePage() {
  return (
    <main className="app-shell min-h-screen px-5 py-8 sm:px-8 lg:px-10">
      <div className="mx-auto flex min-h-[calc(100vh-4rem)] max-w-7xl flex-col gap-8">
        <section className="grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
          <div className="space-y-5">
            <span className="mono inline-flex rounded-full border border-[var(--line-strong)] bg-[rgba(155,214,126,0.08)] px-3 py-1 text-xs uppercase tracking-[0.28em] text-[var(--accent-strong)]">
              Base de producción para CubePath
            </span>
            <div className="space-y-4">
              <h1 className="max-w-4xl text-4xl font-medium tracking-[-0.06em] text-white sm:text-5xl lg:text-7xl">
                Wattless no es solo un auditor; es un reductor de entropía digital.
              </h1>
              <p className="max-w-2xl text-base leading-7 text-[var(--muted)] sm:text-lg">
                Aplicamos principios de termodinámica al desarrollo web para
                que el software deje de calentar el planeta innecesariamente:
                menos bytes, menos coste, mejor UX y un camino claro de mejora.
              </p>
            </div>
          </div>
          <div className="panel flex items-end rounded-[2rem] p-6">
            <div className="space-y-4">
              <p className="mono text-xs uppercase tracking-[0.26em] text-[var(--accent)]">
                Capa de confianza
              </p>
              <p className="text-sm leading-7 text-[var(--muted)]">
                Wattless combina escaneo en runtime con Go, métricas reales de
                Web Performance y recomendaciones accionables para convertir el
                coste invisible de una web en decisiones técnicas concretas.
              </p>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-2xl border border-[var(--line)] bg-[var(--panel-muted)] p-4">
                  <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                    Método
                  </div>
                  <div className="mt-2 text-lg text-white">Sustainable Web Design + LCP</div>
                </div>
                <div className="rounded-2xl border border-[var(--line)] bg-[var(--panel-muted)] p-4">
                  <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                    Infraestructura
                  </div>
                  <div className="mt-2 text-lg text-white">Next.js + Go + CubePath</div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <ScanWorkbench />
      </div>
    </main>
  );
}
