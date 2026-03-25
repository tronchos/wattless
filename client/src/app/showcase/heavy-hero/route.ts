import { readFile } from "node:fs/promises";
import path from "node:path";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  await new Promise((resolve) => setTimeout(resolve, 650));

  const assetPath = path.join(process.cwd(), "public", "showcase", "hero-heavy.bmp");
  const buffer = await readFile(assetPath);

  return new Response(buffer, {
    headers: {
      "Content-Type": "image/bmp",
      "Cache-Control": "no-store",
    },
  });
}
