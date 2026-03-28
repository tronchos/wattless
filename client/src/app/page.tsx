import { ScanWorkbench } from "@/components/scan-workbench";
import { Leaf } from "lucide-react";

export default function HomePage() {
  const footerLinks = ["Documentation", "Privacy Policy", "Carbon Calculator"];

  return (
    <div className="bg-surface text-on-surface min-h-screen selection:bg-primary selection:text-on-primary flex flex-col">
      {/* Minimalist Non-Sticky Branding */}
      <header className="w-full px-6 pt-10 pb-4 max-w-7xl mx-auto flex items-center justify-center sm:justify-between opacity-80 animate-in fade-in duration-1000">
        <div className="text-sm font-bold tracking-widest uppercase font-label text-on-surface-variant flex items-center gap-3">
          <span className="flex h-6 w-6 items-center justify-center rounded bg-primary/10 text-primary">
            <Leaf className="h-3 w-3" />
          </span>
          Digital Biome Auditor
        </div>
        
        {/* Subtle secondary links, hidden on small screens to keep focus */}
        <div className="hidden sm:flex items-center gap-6 text-xs font-label uppercase tracking-widest text-on-surface-variant/60">
          <a className="hover:text-primary transition-colors" href="#methodology">Metodología</a>
          <a className="hover:text-primary transition-colors" href="https://github.com/midudev/hackaton-cubepath-2026" target="_blank" rel="noreferrer">Hackathon</a>
        </div>
      </header>

      <main className="flex-1 max-w-7xl mx-auto px-6 pt-4 pb-24 w-full">
        <ScanWorkbench />
      </main>

      {/* Footer */}
      <footer className="bg-surface-container-lowest border-t border-outline-variant/10 py-12 mt-auto">
        <div className="flex flex-col md:flex-row justify-between items-center px-8 w-full max-w-7xl mx-auto gap-8">
          <div className="space-y-2 text-center md:text-left">
            <div className="text-primary font-bold text-lg font-headline">Wattless by CubePath</div>
            <div className="text-[10px] uppercase tracking-[0.2em] font-label text-on-surface-variant/60">
              © 2026 Digital Biome. Low-carbon audited.
            </div>
          </div>
          <div className="flex flex-wrap justify-center gap-8">
            {footerLinks.map((label) => (
              <span
                key={label}
                title="Próximamente"
                className="text-on-surface-variant text-xs uppercase tracking-widest font-label cursor-default"
              >
                {label}
              </span>
            ))}
          </div>
          <div className="flex items-center gap-4 text-on-surface-variant/40 hover:text-primary transition-colors cursor-pointer">
            <Leaf className="w-5 h-5" />
          </div>
        </div>
      </footer>
    </div>
  );
}
