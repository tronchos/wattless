import { Leaf } from "lucide-react";

import { ScanWorkbench } from "@/components/scan-workbench";

export default function App() {
  return (
    <div className="bg-surface text-on-surface min-h-screen selection:bg-primary selection:text-on-primary flex flex-col">
      <header className="w-full px-6 pt-10 pb-4 max-w-7xl mx-auto flex items-center justify-center sm:justify-between opacity-80 animate-in fade-in duration-1000">
        <div className="text-sm font-bold tracking-widest uppercase font-label text-on-surface-variant flex items-center gap-3">
          <span className="flex h-6 w-6 items-center justify-center rounded bg-primary/10 text-primary">
            <Leaf aria-hidden="true" className="h-3 w-3" />
          </span>
          Wattless
        </div>

        <nav
          aria-label="Navegación principal"
          className="hidden sm:flex items-center gap-6 text-xs font-label uppercase tracking-widest text-on-surface-variant/60"
        >
          <a className="hover:text-primary transition-colors" href="#methodology">
            Metodología
          </a>
        </nav>
      </header>

      <main className="flex-1 max-w-7xl mx-auto px-6 pt-4 pb-24 w-full">
        <ScanWorkbench />
      </main>

      <footer className="bg-surface-container-lowest border-t border-outline-variant/10 py-12 mt-auto">
        <div className="flex flex-col md:flex-row justify-between items-center px-8 w-full max-w-7xl mx-auto gap-8">
          <div className="space-y-2 text-center md:text-left">
            <div className="text-primary font-bold text-lg font-headline">Wattless</div>
            <div className="text-sm text-on-surface-variant max-w-md">
              Auditoría web para entender bytes, CO2 y cuellos de botella reales de rendimiento.
            </div>
          </div>
          <div className="flex flex-wrap justify-center gap-4 text-xs uppercase tracking-widest font-label text-on-surface-variant/70">
            <a className="hover:text-primary transition-colors" href="#scanner">
              Escáner
            </a>
            <a className="hover:text-primary transition-colors" href="#methodology">
              Metodología
            </a>
            <a
              className="hover:text-primary transition-colors"
              href="https://github.com/tronchos/wattless"
              rel="noreferrer"
              target="_blank"
            >
              GitHub
            </a>
          </div>
          <div className="text-[10px] uppercase tracking-[0.2em] font-label text-on-surface-variant/50 text-center md:text-right">
            © 2026 Wattless
          </div>
        </div>
      </footer>
    </div>
  );
}
