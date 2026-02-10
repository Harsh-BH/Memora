import { ReactNode } from "react";

type CTAButtonProps = {
  children: ReactNode;
  href?: string;
  variant?: "primary" | "secondary";
  className?: string;
};

export function CTAButton({
  children,
  href,
  variant = "primary",
  className = "",
}: CTAButtonProps) {
  const baseStyles =
    "inline-flex items-center justify-center rounded-2xl border px-5 py-3 text-sm font-semibold transition-colors";
  const variantStyles =
    variant === "primary"
      ? "border-gray-900 bg-gray-900 text-white hover:bg-gray-800"
      : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50";

  if (href) {
    return (
      <a href={href} className={`${baseStyles} ${variantStyles} ${className}`.trim()}>
        {children}
      </a>
    );
  }

  return (
    <button type="button" className={`${baseStyles} ${variantStyles} ${className}`.trim()}>
      {children}
    </button>
  );
}
