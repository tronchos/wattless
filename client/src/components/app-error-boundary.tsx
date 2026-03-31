import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";

interface AppErrorBoundaryProps {
  children: ReactNode;
}

interface AppErrorBoundaryState {
  error: Error | null;
}

export class AppErrorBoundary extends Component<
  AppErrorBoundaryProps,
  AppErrorBoundaryState
> {
  state: AppErrorBoundaryState = {
    error: null,
  };

  static getDerivedStateFromError(error: Error): AppErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Wattless UI crashed", error, errorInfo);
  }

  reset = () => {
    this.setState({ error: null });
  };

  render() {
    if (!this.state.error) {
      return this.props.children;
    }

    return (
      <div className="flex min-h-screen items-center justify-center px-6">
        <div className="max-w-md text-center space-y-6">
          <AlertTriangle className="w-12 h-12 text-error mx-auto" />
          <h2 className="text-2xl font-bold font-headline text-on-surface">
            Algo salió mal
          </h2>
          <p className="text-sm text-on-surface-variant leading-relaxed">
            {this.state.error.message || "Ocurrió un error inesperado."}
          </p>
          <button
            type="button"
            onClick={this.reset}
            className="bg-primary text-on-primary px-8 py-3 rounded-xl font-bold hover:bg-primary-dim transition-colors text-sm"
          >
            Reintentar
          </button>
        </div>
      </div>
    );
  }
}
