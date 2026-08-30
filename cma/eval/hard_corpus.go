package eval

// hardCorpus replaces the round-3 corpus (20 topically disjoint documents,
// which scored a meaningless 100% recall@k -- any reasonable embedder wins
// a 20-way contest between unrelated paragraphs). This corpus is designed
// to be genuinely hard: documents are grouped into CLUSTERS of 5 on the
// same narrow topic, sharing heavy vocabulary, so a query has to land on
// the one document that actually answers it, not merely the right topic.
//
// Written entirely, corpus and queries both, before any retrieval was run
// against them (see hard_eval_test.go's doc comment for the run log). Not
// adjusted afterward regardless of the result.
var hardCorpus = []string{
	// --- cluster 0: Python memory management (0-4) ---
	"Python's default garbage collector uses reference counting: every object stores a count of references pointing to it, and CPython frees the object the instant that count reaches zero, without waiting for any collection cycle.",
	"Reference counting alone cannot free a group of objects that reference each other but are unreachable from the rest of the program. CPython's cyclic garbage collector runs periodically to find and break these reference cycles.",
	"CPython's cyclic collector is generational: objects are grouped into three generations by age, and younger generations are scanned far more often than older ones, since most objects die young and rarely need to be re-checked.",
	"The Global Interpreter Lock, or GIL, is unrelated to garbage collection despite being confused with it; it is a mutex that lets only one thread execute Python bytecode at a time, protecting the reference counts themselves from race conditions.",
	"A weak reference lets code hold a pointer to an object without increasing its reference count, so the object can still be garbage collected even while the weak reference exists; Python exposes this through the weakref module.",

	// --- cluster 1: marathon running (5-9) ---
	"The modern marathon distance of 42.195 kilometers was standardized in 1921, but the odd figure traces to the 1908 London Olympics, where organizers extended the course so it would finish in front of the royal viewing box.",
	"The men's marathon world record has fallen by more than three minutes since 2000, dropping under two hours and one minute, driven partly by improvements in carbon-plated racing shoes that return more energy per stride.",
	"Qualifying for the Boston Marathon requires runners to hit an age- and gender-specific time in a certified race within the eighteen months before the event, with the fastest qualifying window belonging to men aged eighteen to thirty-four.",
	"Marathon training plans typically taper in the final two to three weeks before race day, sharply reducing weekly mileage while keeping some faster running, so the body arrives at the start line recovered rather than fatigued.",
	"An ultramarathon is any race longer than the standard marathon distance, commonly fifty or one hundred kilometers, and unlike a marathon it usually includes aid stations stocked with solid food rather than just water and gel.",

	// --- cluster 2: ocean and marine geography (10-14) ---
	"The Pacific Ocean is the largest and deepest of Earth's oceans, covering more surface area than all of the planet's land combined and containing roughly half of the planet's total free water volume.",
	"The Mariana Trench in the western Pacific holds the deepest known point in any ocean, the Challenger Deep, which descends to nearly eleven kilometers below sea level, deeper than Mount Everest is tall.",
	"El Nino is a periodic warming of surface waters in the eastern tropical Pacific that disrupts normal wind and rainfall patterns worldwide, typically recurring every two to seven years and lasting nine to twelve months.",
	"Ocean acidification occurs as seawater absorbs excess atmospheric carbon dioxide, lowering its pH and reducing the carbonate ions that shelled organisms like oysters and corals need to build their calcium carbonate structures.",
	"Hydrothermal vents on the deep ocean floor spew mineral-rich, superheated water from cracks in the crust, supporting entire ecosystems of tube worms and bacteria that derive energy from chemicals rather than sunlight.",

	// --- cluster 3: bread baking (15-19) ---
	"A sourdough starter is a live culture of wild yeast and lactic acid bacteria fermenting a mixture of flour and water; it must be fed fresh flour regularly or the culture weakens and eventually dies.",
	"Commercial yeast leavens bread far faster than a sourdough starter because a single strain is bred purely for rapid, reliable carbon dioxide production, whereas a starter's mixed wild culture ferments more slowly but adds tang.",
	"Kneading bread dough develops gluten, the elastic protein network formed when wheat flour's glutenin and gliadin proteins are worked together with water, and it is this network that traps gas and gives bread its chewy structure.",
	"Proofing is the final rise before baking, and both time and temperature matter: a warm kitchen proofs dough in under an hour, while a slow overnight proof in the refrigerator develops deeper flavor from prolonged fermentation.",
	"Rye bread uses flour with less gluten-forming protein than wheat, so rye doughs rise less and stay denser; bakers often blend rye with wheat flour specifically to get enough gluten structure to hold a taller loaf.",

	// --- cluster 4: printing and publishing history (20-24) ---
	"Johannes Gutenberg introduced movable metal type to Europe around 1440, mechanizing text reproduction and sharply cutting the cost of books compared to the hand-copying that had dominated beforehand.",
	"Movable type actually predates Gutenberg by centuries: Bi Sheng developed a ceramic movable type system in China around 1040, and Korea's Goryeo dynasty cast metal type in the 13th century, both well before European printing.",
	"Historians widely credit the printing press with accelerating the Protestant Reformation, since it let Martin Luther's writings spread across Europe far faster than any hand-copied pamphlet could have traveled.",
	"Modern offset printing transfers ink from a plate to a rubber blanket and then onto paper, rather than pressing the plate directly onto the page, which lets the plate last through far higher print volumes.",
	"Early copyright law emerged partly as a response to the printing press: England's Statute of Anne in 1710 was the first law to grant authors, rather than printers, a fixed-term exclusive right to their work.",

	// --- cluster 5: ancient Roman architecture (25-29) ---
	"The Colosseum in Rome could seat an estimated fifty thousand spectators and featured a retractable awning system, operated by sailors, that could be extended over the crowd to provide shade.",
	"Roman aqueducts used gravity alone to move water across long distances, relying on a precisely calculated gentle downward slope over sometimes fifty or more miles rather than any pumping mechanism.",
	"The Pantheon's dome in Rome remains the largest unreinforced concrete dome in the world, achieved by tapering the dome's thickness and mixing progressively lighter volcanic aggregate toward its open central oculus.",
	"The Roman road network eventually spanned over eighty thousand kilometers, built with layered foundations of compacted stone and gravel designed to drain water and support heavy military and trade traffic for centuries.",
	"Roman concrete, made with volcanic ash, lime, and seawater, has proven more durable in marine structures than modern concrete because seawater reacts with the volcanic ash over time to grow strengthening mineral crystals.",

	// --- cluster 6: string instruments (30-34) ---
	"A cello is tuned in fifths from low C to G to D to A, an octave below a viola, and its body is roughly four times the size of a violin's, giving it a much deeper range.",
	"A viola is tuned identically in interval structure to a cello but an octave higher, and is often mistaken for a large violin, though it uses a C string a violin does not have at all.",
	"A double bass is the largest and lowest-pitched instrument in the string family, typically tuned in fourths rather than fifths like the rest of the family, and is usually played standing or on a tall stool.",
	"Classical guitar strings are traditionally nylon for the treble strings and nylon wound with metal for the bass strings, giving a warmer, softer tone than the all-steel strings used on acoustic and electric guitars.",
	"A bow produces sound on a string instrument through stick-slip friction: rosin-coated horsehair grips the string and pulls it sideways until tension overcomes friction, and the string snaps back and repeats the cycle many times a second.",

	// --- cluster 7: economics and inflation (35-39) ---
	"Inflation is the general rise in prices over time, measured by tracking a basket of goods and services, and it erodes the purchasing power of a fixed sum of money the longer that money sits unused.",
	"Hyperinflation refers to inflation exceeding fifty percent a month, a threshold historically reached in interwar Germany and more recently Zimbabwe and Venezuela, usually driven by a government printing money to cover its own spending.",
	"Central banks raise interest rates to fight inflation because higher borrowing costs discourage spending and investment, cooling demand across the economy, though the same rate hikes also risk slowing growth or triggering a recession.",
	"Deflation, a general fall in prices, sounds beneficial but can be economically dangerous because consumers delay purchases expecting further price drops, and debts become effectively more expensive to repay in real terms.",
	"Purchasing power parity compares currencies by the actual cost of a common basket of goods in each country, rather than by market exchange rates, which is why a nominal salary can buy very different lifestyles abroad.",

	// --- cluster 8: CPU architecture (40-44) ---
	"A CPU cache sits between the processor and main memory, storing recently used data in much smaller but far faster memory so the processor is not stalled waiting on slower RAM for every access.",
	"Branch prediction lets a CPU guess which way an upcoming if-statement will go and speculatively execute instructions down that path, discarding the work only if the guess later turns out to be wrong.",
	"Pipelining splits instruction execution into stages, such as fetch, decode, and execute, so that while one instruction is being decoded, the next can already be fetched, overlapping work that would otherwise happen strictly in sequence.",
	"Out-of-order execution lets a CPU run instructions in a different order than the program specifies, as soon as their inputs are ready, then reassembles the results in the original order so the program behaves correctly.",
	"Multicore chips run separate physical execution units in parallel, while multithreading lets a single physical core rapidly interleave more than one instruction stream, which helps hide memory-access stalls but adds no extra arithmetic capacity.",

	// --- cluster 9: hurricanes and tropical storms (45-49) ---
	"A hurricane's eye is a region of calm, often clear weather at the storm's center, typically twenty to forty kilometers wide, distinct from the surrounding eyewall where the storm's strongest winds occur.",
	"The Saffir-Simpson scale ranks hurricanes from category one to category five based purely on sustained wind speed, without directly factoring in rainfall totals or storm surge height, which can vary independently of wind category.",
	"Storm surge, the abnormal rise of seawater pushed ashore by a hurricane's winds, causes more hurricane-related deaths in most storms than wind damage itself, particularly in low-lying coastal areas.",
	"Hurricane names are assigned from a rotating set of alphabetical lists maintained by the World Meteorological Organization, and a name is permanently retired from the list if that storm caused especially severe damage or loss of life.",
	"Hurricane, typhoon, and cyclone all describe the same type of rotating tropical storm; the name used depends only on which ocean basin the storm forms in, not on any difference in the storm's structure.",

	// --- cluster 10: 20th century dystopian literature (50-54) ---
	"George Orwell wrote Nineteen Eighty-Four while suffering from tuberculosis on the remote Scottish island of Jura, finishing the manuscript in 1948, with its title reportedly a reversal of that completion year.",
	"Aldous Huxley's Brave New World imagines a society controlled through engineered happiness and consumption rather than fear, contrasting sharply with the surveillance-and-punishment control depicted in most other dystopian fiction of its era.",
	"Ray Bradbury's Fahrenheit 451 centers on a fireman whose job is burning books, and the title refers to the temperature at which Bradbury was told paper autoignites.",
	"Yevgeny Zamyatin's We, written in 1921 and set in a glass city under total surveillance, is widely considered a direct influence on Orwell, who reviewed it before writing Nineteen Eighty-Four.",
	"Despite their different settings, most 20th-century dystopian novels share recurring tropes: a totalitarian state, suppressed individuality, controlled information, and a protagonist who begins to see through the system's propaganda.",

	// --- cluster 11: canal engineering (55-59) ---
	"The Panama Canal uses a system of locks to raise ships roughly twenty-six meters above sea level to cross the isthmus, then lower them back down using gravity-fed flooding and draining of each chamber.",
	"Unlike the Panama Canal, the Suez Canal uses no locks at all, since it connects the Mediterranean and Red Seas at essentially the same sea level across flat desert terrain.",
	"An earlier French attempt to build the Panama Canal in the 1880s collapsed due to disease, financial mismanagement, and an initial design that lacked locks entirely, before the United States took over construction in 1904.",
	"A 2016 expansion added a third, wider set of locks to the Panama Canal, allowing much larger container ships, sometimes called neo-Panamax vessels, to transit that previously could not fit through the original locks.",
	"The Panama Canal's lock system draws its fresh water from Gatun Lake, an artificial lake created by damming the Chagres River, which also supplies drinking water to nearby Panamanian cities.",

	// --- cluster 12: basic chemistry reactions (60-64) ---
	"Mixing baking soda and vinegar triggers an acid-base reaction that produces carbon dioxide gas, water, and a salt, and the familiar fizzing is simply that gas escaping rapidly as bubbles.",
	"Combustion is a rapid reaction between a fuel and oxygen that releases heat and light, and complete combustion of a hydrocarbon fuel produces only carbon dioxide and water as byproducts.",
	"Rusting is the slow oxidation of iron in the presence of both oxygen and water, forming iron oxide, and it proceeds faster in salty or humid environments because ions in solution accelerate the electrochemical process.",
	"Photosynthesis converts carbon dioxide and water into glucose and oxygen using light energy captured by chlorophyll, and the overall reaction is essentially the reverse of the combustion of glucose in respiration.",
	"An acid-base neutralization reaction in general combines a proton donor and a proton acceptor to form water and a salt, of which the baking-soda-and-vinegar reaction is simply one specific, especially gas-producing example.",

	// --- cluster 13: cognitive biases (65-69) ---
	"Confirmation bias is the tendency to seek out, interpret, and remember information in a way that confirms existing beliefs, while dismissing or forgetting evidence that contradicts them.",
	"Anchoring bias occurs when a person relies too heavily on the first piece of information encountered, such as an initial price, when making subsequent judgments, even when that first number was arbitrary.",
	"The availability heuristic leads people to overestimate the likelihood of events that come to mind easily, such as dramatic plane crashes, while underestimating far more common but less memorable risks like car accidents.",
	"The sunk cost fallacy is the tendency to keep investing time or money into a failing effort because of what has already been spent, rather than evaluating the decision based only on future costs and benefits.",
	"The Dunning-Kruger effect describes how people with low competence in a skill tend to overestimate their own ability, because the same lack of skill that causes poor performance also prevents them from recognizing it.",

	// --- cluster 14: Renaissance and fresco painting (70-74) ---
	"Fresco painting applies pigment directly onto wet plaster so the paint chemically binds with the wall as it dries, forcing the artist to work quickly and in small sections since only wet plaster accepts pigment properly.",
	"Michelangelo painted the Sistine Chapel ceiling while standing on scaffolding, not lying on his back as popularly imagined, and the physical strain reportedly left him unable to read anything except while tilting his head back for months afterward.",
	"Oil paint dries far more slowly than fresco, letting artists blend colors gradually and rework sections over days or weeks, at the cost of the exceptional durability that a properly bonded fresco achieves.",
	"Renaissance painters developed linear perspective, using a single vanishing point and converging lines to create a convincing illusion of three-dimensional depth on a flat wall or panel, a technique earlier medieval art largely lacked.",
	"Restoring an old fresco is especially delicate work because later overpainting, candle soot, and past cleaning attempts must be removed without damaging the original pigment fused into the plaster centuries earlier.",

	// --- cluster 15: agriculture practices (75-79) ---
	"Crop rotation alternates the type of plant grown in a field each season, commonly cycling nitrogen-depleting crops like corn with nitrogen-fixing legumes like soybeans, which also interrupts pest and disease life cycles specific to one crop.",
	"No-till farming leaves the previous season's crop residue on the field instead of plowing it under, reducing soil erosion and preserving soil moisture, though it can require more targeted herbicide use to control weeds.",
	"Drip irrigation delivers water slowly and directly to a plant's root zone through a network of tubes and emitters, using far less water than flood irrigation, which simply covers an entire field with water.",
	"Fertilizers commonly supply nitrogen, phosphorus, and potassium, the three nutrients plants need in the largest amounts, with nitrogen primarily supporting leaf growth and phosphorus supporting root and flower development.",
	"Integrated pest management combines biological controls like beneficial insects, crop rotation, and targeted chemical use only as a last resort, rather than relying on scheduled pesticide spraying regardless of actual pest levels.",

	// --- cluster 16: intellectual property law (80-84) ---
	"A patent grants an inventor the exclusive right to make, use, or sell an invention for a limited period, typically twenty years, in exchange for publicly disclosing exactly how the invention works.",
	"Copyright in most countries now lasts for the life of the author plus seventy years, protecting original creative works like books, music, and films automatically from the moment they are fixed in a tangible form.",
	"A trademark protects a brand identifier, such as a name or logo, and unlike a patent or copyright it can last indefinitely as long as the mark continues to be used commercially and renewed periodically.",
	"A trade secret, unlike a patent, is protected by keeping the information confidential rather than by public disclosure, and that protection lasts only as long as the secret is not independently discovered or leaked.",
	"A patent troll is a company that acquires patents not to build products but purely to extract licensing fees or settlements from other companies through litigation, exploiting the cost of defending a patent lawsuit.",

	// --- cluster 17: coral reef ecology (85-89) ---
	"Coral reefs are built by colonies of tiny animals called polyps, which secrete calcium carbonate skeletons that accumulate over centuries into the large reef structures visible from the surface.",
	"Coral bleaching happens when unusually warm water stresses coral polyps into expelling the symbiotic algae living in their tissue, leaving the coral's white skeleton visible through its now-transparent flesh.",
	"Coral reefs support roughly a quarter of all marine species despite covering under one percent of the ocean floor, making them among the most biodiverse ecosystems per unit area on Earth.",
	"Artificial reefs, built from sunken ships, concrete structures, or purpose-made modules, are deployed to give coral larvae and reef fish a stable surface to colonize where natural reef has been damaged or absent.",
	"Zooxanthellae are the symbiotic algae living inside coral tissue that photosynthesize and supply the coral with most of its energy, which is exactly the partnership that breaks down during a coral bleaching event.",

	// --- cluster 18: diabetes and endocrinology (90-94) ---
	"Insulin is a hormone produced by beta cells in the pancreas that lowers blood glucose by prompting muscle and fat cells to absorb sugar out of the bloodstream.",
	"Type 1 diabetes occurs when the immune system destroys the pancreas's insulin-producing beta cells, while type 2 diabetes develops when the body's cells become resistant to insulin's effect even though insulin is still produced.",
	"Continuous glucose monitors use a small sensor inserted under the skin to track blood sugar levels throughout the day, replacing or supplementing the older method of pricking a fingertip for a manual test strip reading.",
	"Insulin resistance means the body's cells respond less effectively to insulin's signal to absorb glucose, so the pancreas compensates by producing more insulin, a state that can progress to type 2 diabetes over years.",
	"Long-term high blood sugar from poorly controlled diabetes can damage small blood vessels throughout the body, leading to complications including nerve damage, kidney disease, and vision loss if left untreated.",

	// --- cluster 19: historic trade cities (95-99) ---
	"Venice was built on more than a hundred small islands in an Adriatic lagoon, with canals serving as its streets, and its wealth in the medieval period came largely from controlling trade routes between Europe and the East.",
	"Venice's dominance of the spice and silk trade with the East declined sharply after European powers found sea routes around Africa to Asia, bypassing the overland and Mediterranean routes Venice had long controlled.",
	"Dubrovnik, like Venice, built its medieval wealth on maritime trade and diplomacy, maintaining independence for centuries by skillfully balancing relations between larger powers such as Venice itself and the Ottoman Empire.",
	"The Hanseatic League was a confederation of merchant guilds and market towns across northern Europe that dominated Baltic and North Sea trade for centuries, functioning less like a single city and more like a trade alliance.",
	"Trade wealth in cities like Venice funded extensive civic architecture and art patronage, since merchant families competed to display status through grand buildings and commissioned artwork visible to the whole city.",
}

// hardClusterOf maps a corpus index to its cluster id (0-19), five
// documents per cluster in index order.
func hardClusterOf(i int) int { return i / 5 }

type hardQuery struct {
	text     string
	expected int // corpus index, or -1 if no document should be a correct answer
}

// hardQueries: ~5 discriminating queries per cluster (100 total), each
// written to require picking the ONE document in its cluster that answers
// it -- not just the right cluster -- plus queries with no correct answer
// at all, on topics absent from every cluster above.
var hardQueries = []hardQuery{
	// cluster 0: Python memory management
	{"What happens the instant an object's reference count hits zero in CPython?", 0},
	{"Why is a separate collector needed on top of reference counting?", 1},
	{"How does CPython decide how often to scan each generation of objects?", 2},
	{"What does the GIL actually protect, if not memory itself?", 3},
	{"How can code reference an object without stopping it from being collected?", 4},
	// cluster 1: marathon running
	{"Why does the marathon distance end in such an oddly specific number?", 5},
	{"What technology has been credited with the recent drop in marathon world record times?", 6},
	{"Which age group gets the fastest Boston Marathon qualifying time?", 7},
	{"What should a runner's training look like in the weeks right before race day?", 8},
	{"What is served at aid stations in a race longer than a standard marathon?", 9},
	// cluster 2: ocean and marine geography
	{"Roughly what share of Earth's free water does the largest ocean hold?", 10},
	{"How does the deepest ocean trench compare in depth to Mount Everest's height?", 11},
	{"How often does the eastern Pacific warming pattern tend to recur?", 12},
	{"What happens to shelled sea creatures as the ocean absorbs more carbon dioxide?", 13},
	{"What powers ecosystems living around cracks in the deep ocean floor?", 14},
	// cluster 3: bread baking
	{"What happens to a sourdough culture if it isn't fed regularly?", 15},
	{"Why does bread made with commercial yeast rise faster than sourdough?", 16},
	{"What two wheat proteins combine during kneading to trap gas in dough?", 17},
	{"How does an overnight refrigerator rise change a loaf's flavor?", 18},
	{"Why do bakers often mix rye flour with wheat flour instead of using rye alone?", 19},
	// cluster 4: printing and publishing history
	{"Around what year did Gutenberg bring movable metal type to Europe?", 20},
	{"Which two Asian civilizations used movable type before Gutenberg?", 21},
	{"What religious movement is credited to have spread faster because of the printing press?", 22},
	{"What does offset printing transfer ink onto before it reaches the paper?", 23},
	{"What law first gave authors, rather than printers, rights over their own work?", 24},
	// cluster 5: ancient Roman architecture
	{"What device shaded Colosseum spectators from the sun?", 25},
	{"How did Roman aqueducts move water without any pumps?", 26},
	{"What architectural trick let the Pantheon's dome stay standing without steel reinforcement?", 27},
	{"Roughly how many kilometers did the Roman road network eventually cover?", 28},
	{"Why has Roman concrete held up better than modern concrete in seawater?", 29},
	// cluster 6: string instruments
	{"What interval pattern is a cello tuned in?", 30},
	{"What string does a viola have that a violin lacks?", 31},
	{"How is the double bass usually tuned differently from the rest of its family?", 32},
	{"What material difference gives a classical guitar a warmer tone than a steel-string guitar?", 33},
	{"What physical process actually makes a bowed string produce sound?", 34},
	// cluster 7: economics and inflation
	{"How is the general inflation rate actually measured?", 35},
	{"At what monthly rate does inflation get classified as hyperinflation?", 36},
	{"Why do central banks raise interest rates to fight inflation?", 37},
	{"Why can falling prices actually be dangerous for an economy?", 38},
	{"What does purchasing power parity compare that a market exchange rate does not?", 39},
	// cluster 8: CPU architecture
	{"What problem does a cache solve for a processor?", 40},
	{"What does a CPU do while it waits to find out if a branch guess was correct?", 41},
	{"How does splitting instructions into stages let a CPU do more work per cycle?", 42},
	{"How does a CPU keep results correct when instructions run out of program order?", 43},
	{"What is the difference between adding more cores and adding more threads per core?", 44},
	// cluster 9: hurricanes
	{"About how wide is the calm region at a hurricane's center?", 45},
	{"What single factor does the Saffir-Simpson scale rank a hurricane by?", 46},
	{"What part of a hurricane causes the most deaths in many storms?", 47},
	{"When does a hurricane's name get permanently removed from the naming list?", 48},
	{"What actually determines whether a storm is called a hurricane or a typhoon?", 49},
	// cluster 10: dystopian literature
	{"On which island did Orwell finish writing his final novel?", 50},
	{"Which dystopian novel uses engineered pleasure instead of fear to control its society?", 51},
	{"What is the significance of the number in the title Fahrenheit 451?", 52},
	{"Which earlier novel is thought to have directly influenced Orwell's Nineteen Eighty-Four?", 53},
	{"What recurring elements show up across most 20th-century dystopian novels?", 54},
	// cluster 11: canal engineering
	{"How much does the Panama Canal raise ships above sea level?", 55},
	{"Why doesn't the Suez Canal need any locks?", 56},
	{"Why did the first attempt to build the Panama Canal fail?", 57},
	{"What did the 2016 Panama Canal expansion allow through that couldn't fit before?", 58},
	{"Where does the fresh water for the Panama Canal's locks come from?", 59},
	// cluster 12: basic chemistry reactions
	{"What gas is produced when baking soda reacts with vinegar?", 60},
	{"What two byproducts result from completely burning a hydrocarbon fuel?", 61},
	{"Why does iron rust faster in salty air than in dry air?", 62},
	{"What does photosynthesis produce besides glucose?", 63},
	{"What two components combine in any acid-base neutralization reaction?", 64},
	// cluster 13: cognitive biases
	{"What bias makes people mainly notice evidence that supports what they already believe?", 65},
	{"What bias makes an arbitrary first number affect a later decision?", 66},
	{"Why do people overestimate the odds of a dramatic plane crash over a common car accident?", 67},
	{"What fallacy is at play when someone keeps funding a failing project because of money already spent?", 68},
	{"Why do the least skilled people often rate their own ability the highest?", 69},
	// cluster 14: Renaissance and fresco painting
	{"Why must a fresco painter work quickly in small sections?", 70},
	{"What physical toll did painting the Sistine Chapel ceiling reportedly leave on Michelangelo?", 71},
	{"Why does oil paint let an artist blend colors more gradually than fresco does?", 72},
	{"What technique gives Renaissance paintings a convincing illusion of depth?", 73},
	{"What makes restoring an old fresco especially delicate work?", 74},
	// cluster 15: agriculture practices
	{"Why do farmers alternate corn with a nitrogen-fixing crop like soybeans?", 75},
	{"What tradeoff comes with leaving crop residue on a field instead of plowing it under?", 76},
	{"How does drip irrigation use less water than flood irrigation?", 77},
	{"Which nutrient in fertilizer mainly supports a plant's leaf growth?", 78},
	{"How does integrated pest management differ from scheduled pesticide spraying?", 79},
	// cluster 16: intellectual property law
	{"What must an inventor publicly disclose in exchange for a patent?", 80},
	{"How long does copyright typically last after an author's death?", 81},
	{"How is a trademark's potential lifespan different from a patent's?", 82},
	{"How does a trade secret's protection actually work, compared to a patent?", 83},
	{"What is a patent troll's actual business model?", 84},
	// cluster 17: coral reef ecology
	{"What do coral polyps secrete to build a reef's structure?", 85},
	{"What causes a coral's tissue to turn white during a bleaching event?", 86},
	{"What share of marine species do coral reefs support relative to their share of ocean floor?", 87},
	{"What materials are commonly used to build an artificial reef?", 88},
	{"What do the algae living inside coral tissue actually provide to the coral?", 89},
	// cluster 18: diabetes and endocrinology
	{"What effect does insulin have on blood glucose levels?", 90},
	{"What is the core difference between how type 1 and type 2 diabetes develop?", 91},
	{"How does a continuous glucose monitor track blood sugar without finger pricks?", 92},
	{"What happens to insulin production as insulin resistance develops?", 93},
	{"What long-term complications can poorly controlled blood sugar cause?", 94},
	// cluster 19: historic trade cities
	{"What served as the streets in medieval Venice?", 95},
	{"Why did Venice's trade dominance decline after European sea routes to Asia opened?", 96},
	{"How did Dubrovnik maintain independence despite being surrounded by larger powers?", 97},
	{"How did the Hanseatic League differ from a single trading city like Venice?", 98},
	{"What did merchant families in trade cities like Venice spend their wealth on?", 99},

	// --- unanswerable: topics absent from every cluster above ---
	{"What consensus algorithm does Bitcoin use to validate new blocks?", -1},
	{"What triggered the storming of the Bastille in the French Revolution?", -1},
	{"How does a black hole's event horizon relate to its mass?", -1},
	{"What legal test distinguishes an employee from an independent contractor?", -1},
	{"How does a lithium-ion battery's charging speed relate to its degradation?", -1},
	{"What is the offside rule in association football?", -1},
	{"How does DNS resolve a domain name to an IP address?", -1},
	{"What caused the 2008 global financial crisis?", -1},
	{"How does a jet engine generate thrust?", -1},
	{"What is the difference between a virus and a bacterium?", -1},
}
