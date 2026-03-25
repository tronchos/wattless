# Design System Document: The Low-Carbon Aesthetic

## 1. Overview & Creative North Star: "The Digital Biome"
This design system is built for a sustainable web auditor where performance is synonymous with elegance. Our Creative North Star is **"The Digital Biome."** Unlike the aggressive, high-contrast aesthetics of traditional tech, this system mimics the quiet, intentional layers of a forest floor. 

We break the "template" look by rejecting rigid, boxed-in grids. Instead, we utilize **Intentional Asymmetry**—where data visualization sits slightly off-center to invite the eye—and **Tonal Depth** to define hierarchy. This is an editorial approach to utility; it feels like a high-end environmental report rather than a standard SaaS dashboard.

## 2. Colors: Tonal Ecology
The palette is rooted in `surface` (#0D0F0C) and `primary` (#9BD67E), focusing on a "Soft Dark Mode" that reduces OLED power consumption and eye fatigue.

### The "No-Line" Rule
**Standard 1px solid borders are strictly prohibited for sectioning.** Physical boundaries must be defined solely through background color shifts. For example, a `surface-container-low` section should sit on a `surface` background to create a "pocket" of content. This creates a fluid, organic UI that feels carved rather than constructed.

### Surface Hierarchy & Nesting
Treat the UI as a series of nested, physical layers. 
- **Base Level:** `surface` (#0D0F0C).
- **Secondary Content:** `surface-container` (#171B16).
- **Interactive/Floating Elements:** `surface-container-high` (#1C211B).
- **Nested Data Points:** Use `surface-container-lowest` (#000000) inside higher containers to create "sunken" depth for technical data.

### The "Glass & Gradient" Rule
To elevate the "out-of-the-box" feel, use **Glassmorphism** for floating headers or navigation rails. 
- **Token:** `surface-variant` at 60% opacity with a `24px` backdrop-blur.
- **Signature Texture:** For Hero sections or primary CTAs, use a subtle linear gradient from `primary` (#9BD67E) to `primary-container` (#1E5107) at a 135-degree angle. This adds "soul" and depth to the flat mint green.

## 3. Typography: Technical Authority
We pair the geometric quirk of **Space Grotesk** with the utilitarian precision of **IBM Plex Mono**. (Note: While the tokens mention Inter, for this editorial direction, we override body copy with **IBM Plex Mono** to lean into the "auditor" persona).

- **Display & Headline (Space Grotesk):** Use for high-impact environmental stats and section titles. The wide apertures of Space Grotesk feel airy and modern.
- **Body & Data (IBM Plex Mono):** Use for all audit reports and technical descriptions. The monospaced nature emphasizes the "audit" aspect—every character carries weight.
- **Hierarchy Tip:** Use `display-lg` (3.5rem) for singular, impactful metrics (e.g., a "Carbon Score") to create a clear focal point against smaller, descriptive body text.

## 4. Elevation & Depth: Tonal Layering
We do not use shadows to represent "elevation" in the traditional sense. We use **Tonal Layering.**

- **The Layering Principle:** Place a `surface-container-highest` card on a `surface-container-low` background. The difference in gray value provides all the "lift" required.
- **Ambient Shadows:** Only for floating modals. Use a `32px` blur with 8% opacity of the `on-surface` color. It should feel like a soft glow, not a dark drop shadow.
- **The "Ghost Border" Fallback:** If accessibility requires a border, use `outline-variant` (#434A42) at **15% opacity**. It should be felt, not seen.
- **Glassmorphism:** Apply to navigation bars to allow the "low-carbon" content to bleed through as the user scrolls, maintaining a sense of place.

## 5. Components: Editorial Utility

### Buttons
- **Primary:** Gradient fill (`primary` to `primary-container`), `on-primary` text. Border radius: `md` (0.75rem).
- **Secondary:** `surface-variant` fill with `primary` text. No border.
- **Tertiary:** Ghost style. `on-surface` text with a `primary` underline (2px) that appears on hover.

### Chips (Audit Status)
- Use `tertiary-container` for positive "Optimized" states and `error-container` for "High Carbon" states. 
- Shape: `full` (9999px) for a soft, pill-like feel.

### Input Fields
- Fill: `surface-container-highest`. 
- Active State: A 2px bottom-border of `primary` (#9BD67E). Do not wrap the whole box in a color.
- Radius: `sm` (0.25rem).

### Cards & Lists
- **The Divider Rule:** Forbid the use of divider lines. Separate list items using the spacing scale—`3` (1rem) for tight lists and `5` (1.7rem) for editorial lists—combined with subtle alternating background shifts (`surface` to `surface-container-low`).

### Energy Meter (Custom Component)
- A horizontal bar using the `primary_fixed` token as a background track and `primary` as the progress fill. Use `xl` (1.5rem) rounding to make the data feel approachable.

## 6. Do's and Don'ts

### Do:
- **Do** use `16` (5.5rem) and `20` (7rem) spacing for section headers to create "breathing room" typical of high-end journals.
- **Do** use `IBM Plex Mono` for all numerical data to give it an "official audit" feel.
- **Do** utilize the `primary_dim` (#8EC872) for hover states on primary buttons to maintain low visual fatigue.

### Don't:
- **Don't** use pure white (#FFFFFF) for text. Use `on-surface` (#E1E7DD) to keep the contrast "soft."
- **Don't** use standard 1px borders or heavy shadows. This breaks the "Digital Biome" aesthetic.
- **Don't** crowd the layout. If a page feels full, increase the spacing tokens by one tier. High-end design is defined by what you leave out.