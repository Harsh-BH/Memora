const footerColumns = {
  Product: ["Overview", "Integrations", "Security"],
  Company: ["About", "Blog", "Careers"],
  Resources: ["Help center", "Guides", "Status"],
};

export function Footer() {
  return (
    <footer className="border-t border-gray-200 bg-white px-4 py-12 sm:px-6 lg:px-8">
      <div className="mx-auto grid max-w-6xl gap-10 md:grid-cols-[1.2fr,2fr]">
        <div>
          <a href="#" className="text-lg font-bold text-gray-900">
            Memora
          </a>
          <p className="mt-4 max-w-sm text-sm text-gray-600">
            The collaborative memory workspace for teams that move quickly and stay aligned.
          </p>
        </div>
        <div className="grid grid-cols-2 gap-6 sm:grid-cols-3">
          {Object.entries(footerColumns).map(([title, links]) => (
            <div key={title}>
              <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
              <ul className="mt-3 space-y-2 text-sm text-gray-600">
                {links.map((link) => (
                  <li key={link}>
                    <a href="#" className="hover:text-gray-900">
                      {link}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
      <p className="mx-auto mt-10 max-w-6xl border-t border-gray-200 pt-6 text-xs text-gray-500">
        © {new Date().getFullYear()} Memora. All rights reserved.
      </p>
    </footer>
  );
}
