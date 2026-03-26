"use client";

import { AlertTriangle } from "lucide-react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center px-6">
      <div className="max-w-md text-center space-y-6">
        <AlertTriangle className="w-12 h-12 text-error mx-auto" />
        <h2 className="text-2xl font-bold font-headline text-on-surface">
          Algo salió mal
        </h2>
        <p className="text-sm text-on-surface-variant leading-relaxed">
          {error.message || "Ocurrió un error inesperado."}
        </p>
        <button
          onClick={reset}
          className="bg-primary text-on-primary px-8 py-3 rounded-xl font-bold hover:bg-primary-dim transition-colors text-sm"
        >
          Reintentar
        </button>
      </div>
    </div>
  );
}
