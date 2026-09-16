import {courseAreas} from "../data/courseAreas";
import type { APIRoute } from "astro";
import { features } from "../data/features";
import { path } from "../config";
export const GET: APIRoute = ({ site }) => {
  const slugs = [
    "",
    "product",
    "schools",
    "resources",
    "research-studio",
    "about",
    ...courseAreas.map(a=>`courses/${a.id}`),
    ...features.map((f) => `product/${f.id}`),
  ];
  const urls = (["zh", "en"] as const).flatMap((lang) =>
    slugs.map(
      (slug) => `<url><loc>${new URL(path(lang, slug), site)}</loc></url>`,
    ),
  );
  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls.join("")}</urlset>`,
    { headers: { "Content-Type": "application/xml" } },
  );
};
