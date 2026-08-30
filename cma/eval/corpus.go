package eval

// corpus is 20 short, independent passages on distinct topics. Each is
// short enough (under ~120 words) that StructuralSegmenter emits it as a
// single episode, so "the document the query is about" and "the episode
// that should rank highest" are the same thing -- keeping the eval's
// ground truth unambiguous without tuning the corpus toward the model.
//
// Written by hand for this eval, not drawn from any benchmark, and not
// checked against the embedding model before writing -- if a query fails,
// it fails, and that is the honest result this eval is for.
var corpus = []string{
	// 0
	"The Pacific Ocean is the largest and deepest of Earth's five oceans, covering more area than all the planet's land combined. It contains the Mariana Trench, the deepest known point in any ocean, reaching nearly eleven kilometers below sea level. Its size regulates global weather patterns, and its currents drive phenomena like El Nino.",
	// 1
	"Sourdough bread rises without commercial yeast because a starter culture of wild yeast and lactic acid bacteria ferments the flour over days. The bacteria produce acids that give the bread its characteristic tang, while the yeast produces carbon dioxide that leavens the dough. A healthy starter must be fed flour and water regularly or it will weaken.",
	// 2
	"The printing press, developed by Johannes Gutenberg around 1440, used movable metal type to mechanize the reproduction of text. Before this, books were copied by hand, a slow and expensive process. Gutenberg's invention sharply cut the cost of books and is widely credited with accelerating the spread of literacy and the Reformation across Europe.",
	// 3
	"A marathon covers 42.195 kilometers, a distance standardized in 1921 by the International Amateur Athletic Federation. The odd figure traces back to the 1908 London Olympics, where the course was lengthened so it would start at Windsor Castle and finish in front of the royal box at the stadium.",
	// 4
	"Python's garbage collector primarily uses reference counting: every object tracks how many references point to it, and is freed the instant that count reaches zero. A separate cyclic collector periodically scans for groups of objects that reference each other but are unreachable from the rest of the program, since reference counting alone cannot free those.",
	// 5
	"Insulin is a hormone produced by beta cells in the pancreas that lowers blood glucose by prompting muscle and fat cells to absorb sugar from the bloodstream. In type 1 diabetes, the immune system destroys these beta cells, so the body can no longer produce insulin and patients must inject it manually.",
	// 6
	"Venice was built on more than a hundred small islands in a lagoon on the Adriatic coast, with canals serving as its streets. The city's wealth in the medieval and Renaissance periods came largely from controlling trade routes between Europe and the East, particularly in spices and silk carried through its port.",
	// 7
	"A cello is tuned in fifths, from low C to G to D to A, an octave below a viola. Its body is roughly four times the size of a violin's, giving it a much deeper resonant range. Cellists play the instrument upright between their knees, resting its endpin on the floor.",
	// 8
	"Inflation erodes purchasing power over time: if prices rise 3 percent a year, a fixed sum of money buys three percent less each year. Central banks often target a low, steady inflation rate rather than zero, since mild inflation encourages spending and investment over hoarding cash, while runaway inflation destabilizes an economy.",
	// 9
	"The Colosseum in Rome could seat an estimated fifty thousand spectators for gladiatorial contests, mock naval battles, and public executions. Its outer wall used three tiers of arches in different classical architectural orders, and a retractable awning system, operated by sailors, could be extended to shade the crowd from the sun.",
	// 10
	"A CPU cache sits between the processor and main memory, storing recently used data in much faster but much smaller memory so the processor does not stall waiting on slower RAM. Modern chips typically layer several caches, labeled L1 through L3, each larger and slower than the one before it.",
	// 11
	"A hurricane's eye is a region of calm, clear weather at the storm's center, often twenty to forty kilometers wide, surrounded by the eyewall, where the fastest winds and heaviest rain occur. The eye forms because rotating air moving inward is deflected outward by the Coriolis effect before it can fully collapse to the center.",
	// 12
	"George Orwell wrote Nineteen Eighty-Four while suffering from tuberculosis on the remote Scottish island of Jura, finishing the manuscript in 1948. The novel's surveillance state, telescreens, and constant historical revision were shaped by his experience of wartime propaganda and censorship while working for the BBC during the Second World War.",
	// 13
	"The Panama Canal uses a system of locks to raise ships roughly twenty-six meters above sea level to cross the isthmus, then lower them back down on the other side. Each lock chamber floods or drains using gravity alone, moving millions of liters of fresh water from an artificial lake that also supplies drinking water to nearby cities.",
	// 14
	"Baking soda and vinegar react because the base and the acid neutralize each other, producing carbon dioxide gas, water, and a salt. The fizzing many people remember from school science fairs is simply that carbon dioxide escaping rapidly as bubbles, not an explosion, since the reaction is mild and releases little heat.",
	// 15
	"Confirmation bias is the tendency to seek out, interpret, and remember information in a way that confirms what a person already believes, while dismissing or forgetting contradicting evidence. It is considered one of the most persistent cognitive biases because it operates largely outside conscious awareness, even among people trained to recognize it.",
	// 16
	"Fresco painting applies pigment directly onto wet plaster, so the paint binds chemically with the wall as it dries rather than merely sitting on the surface. This gives frescoes remarkable durability across centuries, but the technique forces an artist to work quickly and in sections, since only wet plaster will accept the pigment properly.",
	// 17
	"Crop rotation alternates the type of plant grown in a field each season, commonly cycling between nitrogen-depleting crops like corn and nitrogen-fixing legumes like soybeans or clover. This practice interrupts the life cycles of pests and diseases specific to one crop, while also naturally replenishing soil nutrients instead of relying solely on fertilizer.",
	// 18
	"A patent grants an inventor the exclusive right to make, use, or sell an invention for a limited period, typically twenty years, in exchange for publicly disclosing how the invention works. The bargain is meant to encourage innovation by letting inventors profit from their work, while eventually enriching the public domain once it expires.",
	// 19
	"Coral reefs are built by colonies of tiny animals called polyps, which secrete calcium carbonate skeletons that accumulate over centuries into large reef structures. Reefs support roughly a quarter of all marine species despite covering under one percent of the ocean floor, making them among the most biodiverse ecosystems on Earth.",
}

// query is one labelled (query, expected corpus index) pair. Queries
// paraphrase their source passage rather than reusing its distinctive
// vocabulary verbatim, so a hit reflects semantic retrieval rather than
// literal keyword overlap.
type query struct {
	text     string
	expected int // index into corpus
}

var queries = []query{
	{"Which ocean is the biggest one on the planet?", 0},
	{"What is the deepest point in any ocean called?", 0},
	{"How does an ocean's size affect global weather?", 0},
	{"Why does bread made with a starter taste sour?", 1},
	{"What organisms are responsible for leavening sourdough?", 1},
	{"What happens if you stop feeding a sourdough starter?", 1},
	{"Who invented movable type printing in Europe?", 2},
	{"How did the printing press affect literacy rates?", 2},
	{"What did people do to copy books before printing was invented?", 2},
	{"How long is an official marathon race?", 3},
	{"Why is the marathon distance such an unusual number?", 3},
	{"Which Olympics fixed the modern marathon distance?", 3},
	{"How does Python decide when to free an object from memory?", 4},
	{"What kind of garbage collection problem does reference counting fail to solve?", 4},
	{"Which organ produces the hormone that lowers blood sugar?", 5},
	{"What goes wrong with the pancreas in type 1 diabetes?", 5},
	{"Why do type 1 diabetics need to inject a hormone?", 5},
	{"What served as the roads in medieval Venice?", 6},
	{"How did Venice become wealthy in the Renaissance?", 6},
	{"What goods moved through Venice's trade routes?", 6},
	{"How is a cello different in size from a violin?", 7},
	{"What order are a cello's strings tuned in?", 7},
	{"How does a cellist hold the instrument while playing?", 7},
	{"Why do central banks avoid a zero percent inflation target?", 8},
	{"What happens to money's buying power when prices rise every year?", 8},
	{"How many people could the Colosseum hold at once?", 9},
	{"What system shaded spectators from the sun in the Colosseum?", 9},
	{"What kinds of events took place in the Colosseum?", 9},
	{"Why do modern processors have multiple layers of cache?", 10},
	{"What problem does a CPU cache solve?", 10},
	{"How wide is the calm region at the center of a hurricane?", 11},
	{"Why does a hurricane's eye stay clear instead of collapsing?", 11},
	{"On which island did George Orwell finish writing his famous dystopian novel?", 12},
	{"What personal experiences influenced the surveillance themes in Nineteen Eighty-Four?", 12},
	{"What illness was Orwell suffering from while writing his final novel?", 12},
	{"How much does the Panama Canal raise ships above sea level?", 13},
	{"What powers the flooding and draining of the Panama Canal's locks?", 13},
	{"Where does the canal's fresh water for the locks come from?", 13},
	{"What gas is produced when you mix vinegar and baking soda?", 14},
	{"Why does mixing vinegar and baking soda fizz instead of explode?", 14},
	{"What cognitive bias makes people favor evidence that supports their existing beliefs?", 15},
	{"Why is confirmation bias hard to notice even when you know about it?", 15},
	{"What surface does fresco painting use to bond pigment permanently?", 16},
	{"Why must a fresco painter work quickly in small sections?", 16},
	{"Why do farmers alternate corn with soybeans in the same field?", 17},
	{"How does crop rotation help control pests without more pesticide?", 17},
	{"How long does a patent typically last before it expires?", 18},
	{"What does an inventor have to disclose in exchange for patent protection?", 18},
	{"What tiny animals build the skeletons that form coral reefs?", 19},
	{"What percentage of marine species do coral reefs support despite their small area?", 19},
}
