import { CTAButton } from "@/components/ui/CTAButton";

const navLinks = ["Product", "Features", "Pricing", "Customers"];

export function Navbar() {
  return (
    <header className="sticky top-0 z-20 border-b border-gray-200 bg-white/90 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
        <a href="#" className="text-lg font-bold text-gray-900">
          Memora
        </a>
        <nav className="hidden items-center gap-8 md:flex">
          {navLinks.map((link) => (
            <a key={link} href="#" className="text-sm font-medium text-gray-600 hover:text-gray-900">
              {link}
            </a>
          ))}
        </nav>
        <div className="flex items-center gap-3">
          <a href="#" className="hidden text-sm font-medium text-gray-600 hover:text-gray-900 sm:inline-flex">
            Sign in
          </a>
          <CTAButton href="#">Get Started</CTAButton>
        </div>
      </div>
    </header>
  );
}
