import type { SVGProps } from "react";

export function SrePortalIcon(props: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="80 80 350 350"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <rect x="80" y="80" width="350" height="350" rx="60" fill="#1A1F2C" />

      <path d="M120 180h40v200h-40z" fill="#2D364A" />
      <path d="M180 220h30v160h-30z" fill="#2D364A" />
      <path d="M352 180h40v200h-40z" fill="#2D364A" />
      <path d="M302 220h30v160h-30z" fill="#2D364A" />

      <circle cx="256" cy="256" r="140" stroke="#4A90E2" strokeWidth="12" />
      <circle
        cx="256"
        cy="256"
        r="110"
        stroke="#3A70B2"
        strokeWidth="8"
        strokeDasharray="20 15"
      />

      <path
        d="M256 116v140M256 396v-140M116 256h140M396 256h-140"
        stroke="#4A90E2"
        strokeWidth="6"
      />
      <path
        d="M164 164l92 92M348 348l-92-92M164 348l92-92M348 164l-92 92"
        stroke="#3A70B2"
        strokeWidth="4"
      />

      <circle cx="120" cy="180" r="15" fill="#E67E22" />
      <circle cx="392" cy="180" r="15" fill="#E67E22" />
      <circle cx="256" cy="116" r="15" fill="#27AE60" />
      <circle cx="256" cy="396" r="15" fill="#27AE60" />

      <text
        x="256"
        y="270"
        textAnchor="middle"
        fontFamily="Arial, sans-serif"
        fontSize="60"
        fontWeight="bold"
        fill="white"
        opacity="0.8"
      >
        SRE
      </text>
    </svg>
  );
}
