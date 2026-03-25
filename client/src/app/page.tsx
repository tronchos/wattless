import { ScanWorkbench } from "@/components/scan-workbench";
import { Leaf } from "lucide-react";

export default function HomePage() {
  return (
    <div className="bg-surface text-on-surface min-h-screen selection:bg-primary selection:text-on-primary">
      {/* TopNavBar */}
      <nav className="sticky top-0 z-50 glass-header border-b border-outline-variant/10">
        <div className="flex justify-between items-center w-full px-6 py-4 max-w-screen-2xl mx-auto">
          <div className="text-xl font-bold tracking-tight text-primary flex items-center gap-2">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10">
              <Leaf className="h-5 w-5" />
            </span>
            Digital Biome Auditor
          </div>
          <div className="hidden md:flex items-center gap-8 text-sm uppercase tracking-widest font-label font-bold">
            <a className="text-primary border-b-2 border-primary pb-1" href="#scanner">Reports</a>
            <a className="text-on-surface-variant hover:text-on-surface transition-colors" href="#diagnostic">Diagnostics</a>
            <a className="text-on-surface-variant hover:text-on-surface transition-colors" href="#">Methodology</a>
          </div>
          <div className="flex items-center gap-4">
            <a href="#scanner" className="bg-primary-container text-on-primary-container px-5 py-2 rounded-xl text-sm font-bold hover:bg-primary-container/80 transition-all active:scale-95 duration-200">
              Run Audit
            </a>
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-6 pt-12 pb-24">
        <ScanWorkbench />
      </main>

      {/* Footer */}
      <footer className="bg-surface-container-lowest border-t border-outline-variant/10 py-12 mt-12">
        <div className="flex flex-col md:flex-row justify-between items-center px-8 w-full max-w-7xl mx-auto gap-8">
          <div className="space-y-2 text-center md:text-left">
            <div className="text-primary font-bold text-lg">Digital Biome</div>
            <div className="text-[10px] uppercase tracking-[0.2em] font-label text-on-surface-variant/60">
              © 2026 Digital Biome. Low-carbon audited.
            </div>
          </div>
          <div className="flex flex-wrap justify-center gap-8">
            <a className="text-on-surface-variant hover:text-primary transition-colors text-xs uppercase tracking-widest font-label" href="#">Documentation</a>
            <a className="text-on-surface-variant hover:text-primary transition-colors text-xs uppercase tracking-widest font-label" href="#">Privacy Policy</a>
            <a className="text-on-surface-variant hover:text-primary transition-colors text-xs uppercase tracking-widest font-label" href="#">Carbon Calculator</a>
          </div>
          <div className="flex items-center gap-4 text-on-surface-variant/40">
            <Leaf className="w-6 h-6" />
          </div>
        </div>
      </footer>
    </div>
  );
}
