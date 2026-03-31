package insights

import "strings"

func recommendedFixForFinding(report ReportContext, finding AnalysisFindingContext) *RecommendedFix {
	framework := normalizedFrameworkHint(report.SiteProfile.FrameworkHint)
	switch finding.ID {
	case "render_lcp_candidate", "heavy_above_fold_media":
		return &RecommendedFix{
			Summary:       "Plantilla base para reducir el peso del media crítico y ajustar prioridad de carga sin romper el layout.",
			OptimizedCode: heroMediaOptimizedCode(framework),
			Changes: []string{
				"Dimensiones explícitas para evitar trabajo extra de layout",
				"Calidad controlada y formato moderno para bajar bytes",
				"Prioridad reservada solo al asset crítico de verdad",
			},
			ExpectedImpact: "Menos peso en el render crítico y mejor margen para el LCP.",
		}
	case "render_lcp_dom_node":
		return &RecommendedFix{
			Summary:       "Punto de partida para un LCP textual: fuente disciplinada, CSS estable y menos trabajo bloqueante en el primer render.",
			OptimizedCode: textLCPOtimizedCode(framework),
			Changes: []string{
				"Se reduce incertidumbre tipográfica en el nodo que domina el LCP",
				"Se favorece una pintura estable sin depender de un asset visual pesado",
				"El siguiente paso es revisar CSS crítico y Long Tasks del arranque",
			},
			ExpectedImpact: "Menos espera en el nodo textual que domina el render inicial.",
		}
	case "dominant_image_overdelivery":
		return &RecommendedFix{
			Summary:       "Sirve esta imagen con un tamaño realista para su caja visible y usa una variante moderna cuando el origen siga entregando formatos legacy.",
			OptimizedCode: dominantImageOptimizedCode(framework),
			Changes: []string{
				"Se ajusta el tamaño de salida a la caja visible y a pantallas 2x",
				"Se evita mandar originales de varios megapíxeles a miniaturas pequeñas",
				"Se favorece un formato moderno o una calidad más disciplinada",
			},
			ExpectedImpact: "Gran recorte de bytes en un solo recurso desproporcionado.",
		}
	case "repeated_gallery_overdelivery":
		lazyMajority := repeatedGalleryAlreadyLazy(finding)
		summary := "Patrón para listas visuales repetidas: primeras tarjetas visibles con prioridad y el resto con variantes pequeñas, formatos modernos y carga diferida."
		changes := []string{
			"Eager y prioridad solo para la primera fila visible",
			"Variantes pequeñas, sizes realistas y formatos modernos cuando aplique",
			"Lazy loading para el resto del grid repetido",
		}
		code := repeatedGalleryOptimizedCode(framework, true)
		if lazyMajority {
			summary = "Patrón para listas visuales repetidas ya diferidas: el mayor margen está en sizes/srcset, tamaño de salida y formatos modernos más eficientes."
			changes = []string{
				"Sizes/srcset o equivalente del framework para ajustar cada tarjeta a su caja real",
				"Formatos modernos y calidad disciplinada para no sobreservir miniaturas",
				"Tamaño de salida pensado para la caja visible y para pantallas 2x",
			}
			code = repeatedGalleryOptimizedCode(framework, false)
		}
		return &RecommendedFix{
			Summary:        summary,
			OptimizedCode:  code,
			Changes:        changes,
			ExpectedImpact: "Menor coste por visita sin tocar el primer render.",
		}
	case "third_party_analytics_overhead":
		return &RecommendedFix{
			Summary:       "Patrón conservador para retrasar tags de terceros y evitar que compitan con el arranque.",
			OptimizedCode: deferredAnalyticsCode(framework),
			Changes: []string{
				"Se difiere el proveedor hasta que la página ya cargó",
				"Se reduce ruido de red durante el render inicial",
			},
			ExpectedImpact: "Menos variabilidad y menos presión de terceros en el arranque.",
		}
	case "third_party_social_overhead":
		return &RecommendedFix{
			Summary:       "Carga diferida para embeds sociales: placeholder estático primero y script externo solo cuando haya interacción o el bloque entre en viewport.",
			OptimizedCode: deferredSocialEmbedCode(framework),
			Changes: []string{
				"Se evita descargar widgets sociales durante el arranque",
				"El embed real solo se activa cuando el usuario lo necesita",
			},
			ExpectedImpact: "Menos ruido de terceros y menos JS externo en la carga inicial.",
		}
	case "third_party_payment_overhead":
		return &RecommendedFix{
			Summary:       "Carga bajo demanda para ticketing o pagos: placeholder primero y widget real solo cuando el usuario quiera comprar o abrir checkout.",
			OptimizedCode: deferredPaymentEmbedCode(framework),
			Changes: []string{
				"Se evita montar el widget de pago durante el arranque",
				"El tercero solo entra cuando la intención del usuario lo justifica",
			},
			ExpectedImpact: "Menos iframes y menos red de terceros antes de la interacción.",
		}
	case "third_party_video_overhead":
		return &RecommendedFix{
			Summary:       "Video diferido con poster ligero: miniatura primero y player/iframe real solo al interactuar o al entrar en viewport.",
			OptimizedCode: deferredVideoEmbedCode(framework),
			Changes: []string{
				"Se evita descargar el player externo en el arranque",
				"El embed real solo aparece cuando el usuario lo necesita",
			},
			ExpectedImpact: "Menos peso de terceros y menos scripts/iframes antes de la reproducción.",
		}
	case "third_party_ads_overhead":
		return &RecommendedFix{
			Summary:       "Recorta el stack publicitario temprano: limita auctions, posterga iframes/slots no críticos y reduce vendors en el arranque.",
			OptimizedCode: deferredAdsCode(framework),
			Changes: []string{
				"Se evita montar slots o subastas no esenciales durante el arranque",
				"Se reduce la presión de scripts e iframes externos antes del contenido editorial",
			},
			ExpectedImpact: "Menos variabilidad, menos trabajo de terceros y menos bytes antes de la interacción.",
		}
	case "font_stack_overweight":
		if reportHasIconFont(report, finding) {
			return &RecommendedFix{
				Summary:       "Sustituye la fuente de iconos por SVGs individuales o genera un subset mínimo si todavía necesitas servirla como font.",
				OptimizedCode: iconFontOptimizedCode(framework),
				Changes: []string{
					"Se eliminan miles de glifos no usados de una icon font genérica",
					"Los iconos críticos pasan a SVGs ligeros y explícitos",
				},
				ExpectedImpact: "Menos peso tipográfico y menos dependencia de una font de iconos sobredimensionada.",
			}
		}
		return &RecommendedFix{
			Summary: "Punto de partida para limitar variantes y servir una pila tipográfica más pequeña.",
			OptimizedCode: `@font-face {
  font-family: "Brand Sans";
  src: url("/fonts/brand-sans-subset.woff2") format("woff2");
  font-display: swap;
  font-weight: 400 700;
}`,
			Changes: []string{
				"Subset de glifos y una sola variante moderna",
				"font-display: swap para no bloquear contenido",
			},
			ExpectedImpact: "Menos transferencia tipográfica y render inicial más estable.",
		}
	case "main_thread_cpu_pressure":
		return &RecommendedFix{
			Summary:       "Ejemplo de importación diferida para bajar trabajo de CPU del arranque.",
			OptimizedCode: deferredCPUCode(framework),
			Changes: []string{
				"El código pesado deja de competir con la hebra principal al inicio",
				"Se aísla JS costoso fuera del camino crítico",
			},
			ExpectedImpact: "Menos Long Tasks y mejor respuesta percibida.",
		}
	case "responsive_image_overdelivery":
		return &RecommendedFix{
			Summary:       "Sirve una imagen adaptada a su caja renderizada y declara variantes para no mandar desktop completo a cajas pequeñas.",
			OptimizedCode: responsiveImageCode(framework, finding),
			Changes: []string{
				"srcset/sizes o equivalente del framework para ajustar el tamaño real",
				"Variantes pensadas para la caja visible y para pantallas 2x",
				"Menos bytes sin perder nitidez perceptible",
			},
			ExpectedImpact: "Menos transferencia por imagen sin degradar la experiencia visual.",
		}
	case "legacy_image_format_overhead":
		return &RecommendedFix{
			Summary:       "Sirve variantes AVIF/WebP y deja JPEG/PNG solo como fallback donde de verdad haga falta.",
			OptimizedCode: legacyImageFormatCode(framework),
			Changes: []string{
				"Se priorizan formatos modernos para la mayoría del catálogo visual",
				"Se mantienen fallbacks legacy solo donde la compatibilidad lo exige",
			},
			ExpectedImpact: "Menos peso de imagen acumulado sin degradar la calidad percibida.",
		}
	case "legacy_font_format_overhead":
		return &RecommendedFix{
			Summary:       "Sirve WOFF2 como formato principal para fuentes de texto y deja WOFF/TTF solo como fallback excepcional.",
			OptimizedCode: legacyFontFormatCode(framework),
			Changes: []string{
				"Se reduce el peso tipográfico con un formato más eficiente",
				"Se evita seguir pagando bytes de formatos legacy en navegadores modernos",
			},
			ExpectedImpact: "Menos transferencia tipográfica sin cambiar la identidad visual.",
		}
	default:
		return nil
	}
}
func reportHasIconFont(report ReportContext, finding AnalysisFindingContext) bool {
	if findingSuggestsIconFont(finding) {
		return true
	}

	related := make(map[string]struct{}, len(finding.RelatedResourceIDs))
	for _, id := range finding.RelatedResourceIDs {
		related[id] = struct{}{}
	}
	for _, asset := range report.TopResources {
		if len(related) > 0 {
			if _, ok := related[asset.ID]; !ok {
				continue
			}
		}
		if isIconFontResourceContext(asset) {
			return true
		}
	}
	if len(finding.RelatedResourceIDs) == 0 {
		for _, asset := range report.TopResources {
			if isIconFontResourceContext(asset) {
				return true
			}
		}
	}
	return false
}
func findingSuggestsIconFont(finding AnalysisFindingContext) bool {
	haystack := strings.ToLower(strings.TrimSpace(
		finding.Title + " " + finding.Summary + " " + strings.Join(finding.Evidence, " "),
	))
	return containsAny(haystack, "fuente de iconos", "icon font", "font awesome", "fa-solid", "material icons")
}
func normalizedFrameworkHint(value string) string {
	switch value {
	case "astro", "nextjs", "generic", "unknown":
		return value
	default:
		return "generic"
	}
}
func heroMediaOptimizedCode(framework string) string {
	switch framework {
	case "astro":
		return `---
import { Image } from "astro:assets";
import hero from "../assets/hero.webp";
---

<Image
  src={hero}
  alt="Asset crítico"
  widths={[640, 960, 1280]}
  sizes="100vw"
  loading="eager"
  fetchpriority="high"
  format="avif"
/>`
	case "generic", "unknown":
		return `<img
  src="/asset-critico-1280.webp"
  srcset="/asset-critico-640.webp 640w, /asset-critico-960.webp 960w, /asset-critico-1280.webp 1280w"
  sizes="100vw"
  width="1280"
  height="720"
  loading="eager"
  fetchpriority="high"
  alt="Asset crítico"
/>`
	default:
		return `import Image from "next/image";

export function HeroAsset() {
  return (
    <Image
      src="/asset-critico.webp"
      alt="Asset crítico"
      width={1280}
      height={720}
      priority
      sizes="100vw"
      quality={72}
    />
  );
}`
	}
}
func textLCPOtimizedCode(framework string) string {
	if framework == "nextjs" {
		return `import { Inter } from "next/font/google";

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  preload: true,
});

export function HeroCopy() {
  return (
    <section>
      <h1 className={inter.className}>Contenido crítico listo para pintar</h1>
    </section>
  );
}`
	}

	return `@font-face {
  font-family: "Brand Sans";
  src: url("/fonts/brand-sans-subset.woff2") format("woff2");
  font-display: swap;
  font-weight: 400 700;
}

.hero-title {
  font-family: "Brand Sans", system-ui, sans-serif;
}`
}
func repeatedGalleryAlreadyLazy(finding AnalysisFindingContext) bool {
	for _, evidence := range finding.Evidence {
		if strings.Contains(strings.ToLower(strings.TrimSpace(evidence)), `lazy loading ya presente en la mayoría`) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(finding.Summary), "ya usan lazy loading")
}
func repeatedGalleryOptimizedCode(framework string, includeLoading bool) string {
	switch framework {
	case "astro":
		loadingLine := `    loading={index < firstRowCount ? "eager" : "lazy"}
    fetchpriority={index < firstRowCount ? "high" : "auto"}`
		if !includeLoading {
			loadingLine = ""
		}
		return `---
import { Image } from "astro:assets";
const firstRowCount = Math.max(1, Astro.props.firstRowCount ?? 1);
---

{items.map((item, index) => (
  <Image
    src={item.image}
    alt={item.title}
    widths={[320, 480, 640]}
    sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
    format="avif"
` + loadingLine + `
  />
))}`
	case "generic", "unknown":
		loadingLine := `      'loading="' + (isVisibleRow ? "eager" : "lazy") + '" ' +
      'fetchpriority="' + (isVisibleRow ? "high" : "auto") + '" ' +`
		if !includeLoading {
			loadingLine = ""
		}
		return `function renderCardGrid(items, options) {
  options = options || {};
  const firstRowCount = Math.max(1, options.firstRowCount || 1);
  return items.map(function (item, index) {
    const isVisibleRow = index < firstRowCount;
    return '<img ' +
      'src="' + item.image480 + '" ' +
      'srcset="' + item.image320 + ' 320w, ' + item.image480 + ' 480w, ' + item.image640 + ' 640w" ' +
      'sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw" ' +
      'width="480" height="270" ' +
` + loadingLine + `
      'alt="' + item.title + '">' ;
  }).join("");
}`
	default:
		loadingLine := `      loading={index < firstRowCount ? "eager" : "lazy"}
      fetchPriority={index < firstRowCount ? "high" : "auto"}`
		if !includeLoading {
			loadingLine = ""
		}
		return `import Image from "next/image";

export function CardGrid({ items, firstRowCount = 1 }) {
  return items.map((item, index) => (
    <Image
      key={item.id ?? item.slug ?? item.href ?? index}
      src={item.image}
      alt={item.title}
      width={480}
      height={270}
      sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
      quality={68}
` + loadingLine + `
    />
  ));
}`
	}
}
func dominantImageOptimizedCode(framework string) string {
	switch framework {
	case "astro":
		return `---
import { Image } from "astro:assets";
---

<Image
  src={Astro.props.image}
  alt={Astro.props.title}
  widths={[288, 576, 864]}
  sizes="(max-width: 768px) 100vw, 288px"
  format="avif"
  quality={68}
/>`
	case "generic", "unknown":
		return `<img
  src="/images/card-576.avif"
  srcset="/images/card-288.avif 288w, /images/card-576.avif 576w, /images/card-864.avif 864w"
  sizes="(max-width: 768px) 100vw, 288px"
  width="288"
  height="114"
  alt="Miniatura optimizada"
  loading="lazy">`
	default:
		return `import Image from "next/image";

export function CardThumbnail({ item }) {
  return (
    <Image
      src={item.image}
      alt={item.title}
      width={576}
      height={228}
      sizes="(max-width: 768px) 100vw, 288px"
      quality={68}
    />
  );
}`
	}
}
func deferredAnalyticsCode(framework string) string {
	if framework == "nextjs" {
		return `import Script from "next/script";

export function DeferredAnalytics() {
  return (
    <Script
      src="https://analytics.example.com/tag.js"
      strategy="lazyOnload"
    />
  );
}`
	}

	return `<script>
  window.addEventListener("load", () => {
    requestIdleCallback(() => {
      const script = document.createElement("script");
      script.src = "https://analytics.example.com/tag.js";
      script.async = true;
      document.head.appendChild(script);
    });
  });
</script>`
}
func deferredSocialEmbedCode(framework string) string {
	if framework == "nextjs" {
		return `import { useState } from "react";
import Script from "next/script";

export function SocialEmbed() {
  const [enabled, setEnabled] = useState(false);

  return (
    <section>
      {!enabled ? (
        <button onClick={() => setEnabled(true)}>
          Cargar publicación
        </button>
      ) : null}
      {enabled ? (
        <Script
          src="https://social.example.com/embed.js"
          strategy="lazyOnload"
        />
      ) : null}
      <div data-social-embed />
    </section>
  );
}`
	}

	return `<section>
  <button type="button" data-load-social>
    Cargar publicación
  </button>
  <div data-social-embed></div>
</section>
<script>
  document.querySelector("[data-load-social]")?.addEventListener("click", () => {
    const script = document.createElement("script");
    script.src = "https://social.example.com/embed.js";
    script.async = true;
    document.head.appendChild(script);
  }, { once: true });
</script>`
}
func deferredPaymentEmbedCode(framework string) string {
	if framework == "nextjs" {
		return `import { useState } from "react";

export function TicketingCTA() {
  const [enabled, setEnabled] = useState(false);

  return (
    <section>
      {!enabled ? (
        <button onClick={() => setEnabled(true)}>
          Ver entradas
        </button>
      ) : null}
      {enabled ? (
        <iframe
          src="https://buy.example.com/widget"
          title="Compra tus entradas"
          loading="lazy"
        />
      ) : null}
    </section>
  );
}`
	}

	return `<section>
  <button type="button" data-open-ticketing>
    Ver entradas
  </button>
  <div data-ticketing-slot></div>
</section>
<script>
  document.querySelector("[data-open-ticketing]")?.addEventListener("click", () => {
    const iframe = document.createElement("iframe");
    iframe.src = "https://buy.example.com/widget";
    iframe.loading = "lazy";
    iframe.title = "Compra tus entradas";
    document.querySelector("[data-ticketing-slot]")?.appendChild(iframe);
  }, { once: true });
</script>`
}
func deferredVideoEmbedCode(framework string) string {
	if framework == "nextjs" {
		return `import { useState } from "react";

export function VideoEmbed({ poster }) {
  const [enabled, setEnabled] = useState(false);

  return (
    <section>
      {!enabled ? (
        <button onClick={() => setEnabled(true)}>
          <img src={poster} alt="Poster del video" />
          <span>Reproducir video</span>
        </button>
      ) : null}
      {enabled ? (
        <iframe
          src="https://www.youtube.com/embed/example"
          title="Video diferido"
          loading="lazy"
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowFullScreen
        />
      ) : null}
    </section>
  );
}`
	}

	return `<section>
  <button type="button" data-play-video>
    <img src="/poster-lightweight.webp" alt="Poster del video">
    <span>Reproducir video</span>
  </button>
  <div data-video-slot></div>
</section>
<script>
  document.querySelector("[data-play-video]")?.addEventListener("click", () => {
    const iframe = document.createElement("iframe");
    iframe.src = "https://www.youtube.com/embed/example";
    iframe.loading = "lazy";
    iframe.title = "Video diferido";
    iframe.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture";
    iframe.allowFullscreen = true;
    document.querySelector("[data-video-slot]")?.appendChild(iframe);
  }, { once: true });
</script>`
}
func iconFontOptimizedCode(framework string) string {
	if framework == "nextjs" {
		return `import { CheckCircle, CalendarDays } from "lucide-react";

export function FeatureList() {
  return (
    <ul>
      <li><CheckCircle size={18} /> Agenda publicada</li>
      <li><CalendarDays size={18} /> Entradas disponibles</li>
    </ul>
  );
}`
	}

	return `<ul class="feature-list">
  <li>
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M20 6 9 17l-5-5" fill="none" stroke="currentColor" stroke-width="2"/>
    </svg>
    Agenda publicada
  </li>
  <li>
    <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M7 2v3M17 2v3M4 8h16M5 5h14a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1Z" fill="none" stroke="currentColor" stroke-width="2"/>
    </svg>
    Entradas disponibles
  </li>
</ul>`
}
func deferredCPUCode(framework string) string {
	if framework == "nextjs" {
		return `import dynamic from "next/dynamic";

const HeavyWidget = dynamic(() => import("./heavy-widget"), {
  ssr: false,
});

export function DeferredWidget() {
  return <HeavyWidget />;
}`
	}

	return `<script type="module">
  requestIdleCallback(async () => {
    const { mountHeavyWidget } = await import("/scripts/heavy-widget.js");
    mountHeavyWidget(document.querySelector("[data-heavy-widget]"));
  });
</script>`
}
func responsiveImageCode(framework string, finding AnalysisFindingContext) string {
	loadingAttr := `loading="lazy"`
	if responsiveImageShouldEager(finding) {
		loadingAttr = `loading="eager"`
	}
	switch framework {
	case "astro":
		return `---
import { Image } from "astro:assets";
import cardCover from "../assets/card-cover.webp";
---

<Image
  src={cardCover}
  alt="Portada optimizada"
  widths={[320, 480, 640]}
  sizes="(max-width: 768px) 100vw, 480px"
  ` + loadingAttr + `
/>`
	case "generic", "unknown":
		return `<img
  src="/card-cover-480.webp"
  srcset="/card-cover-320.webp 320w, /card-cover-480.webp 480w, /card-cover-640.webp 640w"
  sizes="(max-width: 768px) 100vw, 480px"
  width="480"
  height="270"
  ` + loadingAttr + `
  alt="Portada optimizada"
/>`
	default:
		if loadingAttr == `loading="eager"` {
			return `import Image from "next/image";

export function ResponsiveCardImage() {
  return (
    <Image
      src="/card-cover.webp"
      alt="Portada optimizada"
      width={480}
      height={270}
      sizes="(max-width: 768px) 100vw, 480px"
      loading="eager"
    />
  );
}`
		}
		return `import Image from "next/image";

export function ResponsiveCardImage() {
  return (
    <Image
      src="/card-cover.webp"
      alt="Portada optimizada"
      width={480}
      height={270}
      sizes="(max-width: 768px) 100vw, 480px"
      loading="lazy"
    />
  );
}`
	}
}
func responsiveImageShouldEager(finding AnalysisFindingContext) bool {
	haystack := strings.ToLower(strings.TrimSpace(finding.Summary + " " + strings.Join(finding.Evidence, " ")))
	return containsAny(haystack, "above fold", "above_fold", "hero_media", "hero media", "lcp")
}
func legacyImageFormatCode(framework string) string {
	switch framework {
	case "nextjs":
		return `import Image from "next/image";

export function CoverImage() {
  return (
    <Image
      src="/cover.avif"
      alt="Portada optimizada"
      width={1280}
      height={720}
      sizes="(max-width: 768px) 100vw, 1280px"
    />
  );
}`
	case "astro":
		return `---
import { Picture } from "astro:assets";
import cover from "../assets/cover.jpg";
---

<Picture
  src={cover}
  alt="Portada optimizada"
  formats={["avif", "webp"]}
  widths={[480, 768, 1280]}
  sizes="(max-width: 768px) 100vw, 1280px"
/>`
	default:
		return `<picture>
  <source srcset="/cover.avif" type="image/avif">
  <source srcset="/cover.webp" type="image/webp">
  <img
    src="/cover.jpg"
    width="1280"
    height="720"
    loading="lazy"
    alt="Portada optimizada"
  >
</picture>`
	}
}
func legacyFontFormatCode(framework string) string {
	_ = framework
	return `@font-face {
  font-family: "Brand Sans";
  src:
    url("/fonts/brand-sans.woff2") format("woff2"),
    url("/fonts/brand-sans.woff") format("woff");
  font-display: swap;
  font-weight: 400 700;
}`
}
func deferredAdsCode(framework string) string {
	if framework == "nextjs" {
		return `import { useEffect, useState } from "react";

export function DeferredAdSlot() {
  const [shouldMount, setShouldMount] = useState(false);

  useEffect(() => {
    const timer = window.setTimeout(() => setShouldMount(true), 2500);
    return () => window.clearTimeout(timer);
  }, []);

  return shouldMount ? <div id="ad-slot-top" /> : <div style={{ minHeight: 250 }} />;
}`
	}

	return `<div id="ad-slot-top" style="min-height:250px"></div>
<script>
  window.addEventListener("load", () => {
    requestIdleCallback(() => {
      if (window.googletag) {
        window.googletag.cmd.push(() => {
          window.googletag.display("ad-slot-top");
        });
      }
    });
  });
</script>`
}
