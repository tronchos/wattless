package insights

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type RuleBasedProvider struct{}

func NewRuleBasedProvider() RuleBasedProvider {
	return RuleBasedProvider{}
}

func (RuleBasedProvider) Name() string {
	return "rule_based"
}

func (RuleBasedProvider) SuggestResource(resource ResourceContext) string {
	resourceType := strings.ToLower(resource.Type)
	switch {
	case resource.Failed:
		return "Corrige el fallo de esta petición o elimina la dependencia si ya no aporta valor."
	case resourceType == "video":
		return "Transcodifica este video, reduce bitrate y evita autoplay si no es esencial."
	case resourceType == "image" && resource.Bytes > 500_000:
		return "Comprime, recorta o convierte esta imagen a AVIF/WebP para bajar peso y mejorar el LCP."
	case resourceType == "script" && resource.Bytes > 250_000:
		return "Divide este bundle, difiere código no crítico y retrasa terceros hasta interacción."
	case resourceType == "stylesheet":
		return "Elimina CSS no usado e inyecta solo estilos críticos en el primer render."
	case resourceType == "font":
		return "Subconjunta la fuente, limita variantes y sirve WOFF2."
	default:
		return "Reduce transferencia o carga este recurso de forma diferida."
	}
}

func (provider RuleBasedProvider) SummarizeReport(_ context.Context, report ReportContext) (ScanInsights, error) {
	actions := make([]TopAction, 0, 3)
	for index, resource := range prioritizeResources(report.TopResources) {
		if index >= 3 {
			break
		}

		title, reason, impact := provider.actionCopy(resource, report)
		actions = append(actions, TopAction{
			ID:                    fmt.Sprintf("act-%d", index+1),
			Title:                 title,
			Reason:                reason,
			EstimatedSavingsBytes: resource.EstimatedSavingsBytes,
			LikelyLCPImpact:       impact,
			RelatedResourceID:     resource.ID,
		})
	}

	summary := "Tu web ya ofrece una auditoría útil, pero todavía hay margen para reducir bytes y mejorar la experiencia percibida."
	switch {
	case report.Performance.LCPMS > 2500 && len(report.TopResources) > 0:
		summary = "Tu sostenibilidad está frenada por el render inicial: el LCP es alto y los recursos principales siguen cargando demasiado peso."
	case report.Summary.ThirdPartyBytes > report.Summary.FirstPartyBytes:
		summary = "Los recursos de terceros dominan la transferencia. Reducirlos bajará CO2, ruido de red y variabilidad en la carga."
	case report.CO2GramsPerVisit <= 0.10:
		summary = "La base es buena: la página ya es ligera, pero aún puedes afinar el render crítico para hacerla más consistente."
	}

	pitchLine := "Menos bytes y menos bloqueo en el render significan menos CO2, mejor LCP y una experiencia más rápida para la persona usuaria."
	if len(actions) > 0 && actions[0].LikelyLCPImpact == "high" {
		pitchLine = "Atacando el recurso principal y retrasando lo no crítico puedes mejorar el LCP y bajar la huella por visita en el mismo movimiento."
	}

	return ScanInsights{
		Provider:         provider.Name(),
		ExecutiveSummary: summary,
		PitchLine:        pitchLine,
		TopActions:       actions,
	}, nil
}

func (provider RuleBasedProvider) RefactorCode(_ context.Context, request RefactorRequest) (RefactorResult, error) {
	framework := strings.ToLower(strings.TrimSpace(request.Framework))
	language := strings.ToLower(strings.TrimSpace(request.Language))

	optimizedCode := `import Image from "next/image";
import Script from "next/script";

type HeroProps = {
  title: string;
  subtitle: string;
};

export function Hero({ title, subtitle }: HeroProps) {
  return (
    <section className="grid gap-6 lg:grid-cols-[1.1fr_0.9fr] lg:items-center">
      <div className="space-y-4">
        <p className="text-sm uppercase tracking-[0.24em] text-emerald-300">
          Rendimiento sostenible
        </p>
        <h1 className="text-4xl font-semibold tracking-tight text-white sm:text-5xl">
          {title}
        </h1>
        <p className="max-w-2xl text-base leading-7 text-slate-300">
          {subtitle}
        </p>
      </div>

      <div className="relative overflow-hidden rounded-3xl border border-white/10 bg-slate-950">
        <Image
          src="/showcase/hero-wattless.svg"
          alt="Vista optimizada de la hero"
          width={1200}
          height={900}
          priority
          sizes="(max-width: 768px) 100vw, 48vw"
          className="h-auto w-full"
        />
      </div>

      <Script
        src="/showcase/wattless-idle.js"
        strategy="lazyOnload"
      />
    </section>
  );
}`

	if framework != "next" && framework != "react" {
		optimizedCode = `export function optimizeHero(container) {
  const image = container.querySelector("img");
  if (image) {
    image.loading = "lazy";
    image.decoding = "async";
    image.width = image.width || 1200;
    image.height = image.height || 900;
  }

  const thirdPartyScripts = container.querySelectorAll("script[data-non-critical='true']");
  for (const script of thirdPartyScripts) {
    script.defer = true;
  }
}`
	}

	if language == "jsx" {
		optimizedCode = strings.ReplaceAll(optimizedCode, "type HeroProps = {\n  title: string;\n  subtitle: string;\n};\n\n", "")
		optimizedCode = strings.ReplaceAll(optimizedCode, "({ title, subtitle }: HeroProps)", "({ title, subtitle })")
	}

	changes := []string{
		"Uso de un asset optimizado para el bloque principal.",
		"`sizes` y dimensiones explícitas para reducir trabajo de layout.",
		"Carga diferida del script no crítico para liberar el render inicial.",
	}

	expectedImpact := "Reduce transferencia en el hero, hace más estable el render principal y suele mejorar el LCP sin empeorar la experiencia."
	if request.ReportContext.LCPMS > 0 {
		expectedImpact = fmt.Sprintf("Ataca el cuello de botella del render principal. Con un LCP actual de %d ms, esta refactorización debería recortar bytes y mejorar la percepción de velocidad.", request.ReportContext.LCPMS)
	}

	return RefactorResult{
		Provider:       provider.Name(),
		Summary:        "Se prioriza el contenido crítico, se optimiza el asset principal y se retrasa el JavaScript no esencial.",
		OptimizedCode:  optimizedCode,
		Changes:        changes,
		ExpectedImpact: expectedImpact,
	}, nil
}

func (provider RuleBasedProvider) actionCopy(resource ResourceContext, report ReportContext) (title string, reason string, impact string) {
	impact = "medium"
	resourceType := strings.ToLower(resource.Type)

	switch {
	case resource.Failed:
		title = "Elimina o corrige una petición fallida"
		reason = "Sigue generando ruido de red y complejidad sin aportar valor al usuario final."
		impact = "low"
	case resourceType == "image":
		title = "Optimiza la imagen principal"
		reason = "Aporta mucho peso y es muy probable que influya en el render crítico."
		if report.Performance.LCPMS >= 2000 {
			impact = "high"
		}
	case resourceType == "script":
		title = "Retrasa JavaScript no crítico"
		reason = "El bundle principal compite con el render, aumenta CPU y retrasa interactividad."
		if report.Performance.ScriptDurationMS > 200 {
			impact = "high"
		}
	case resourceType == "video":
		title = "Reduce el costo del video"
		reason = "El video domina la transferencia y dispara el costo por visita."
		impact = "high"
	case resourceType == "font":
		title = "Recorta la carga de tipografías"
		reason = "Las fuentes pesadas penalizan el primer render y añaden peticiones evitables."
	default:
		title = "Reduce el peso del recurso dominante"
		reason = "Es uno de los elementos con mayor transferencia dentro de la página analizada."
	}

	if resource.TransferShare >= 20 {
		reason = "Este recurso concentra una parte muy alta de la transferencia total y es el mejor punto de ataque para una mejora rápida."
	}

	return title, reason, impact
}

func prioritizeResources(resources []ResourceContext) []ResourceContext {
	sorted := append([]ResourceContext(nil), resources...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].EstimatedSavingsBytes == sorted[j].EstimatedSavingsBytes {
			return sorted[i].Bytes > sorted[j].Bytes
		}
		return sorted[i].EstimatedSavingsBytes > sorted[j].EstimatedSavingsBytes
	})
	return sorted
}
