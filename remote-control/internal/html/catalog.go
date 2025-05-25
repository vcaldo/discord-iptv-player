package html

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/vcaldo/discord-iptv-player/remote_control/internal/models"
)

type CatalogGenerator struct{}

type CatalogData struct {
	PlaylistName     string
	Date             string
	Categories       []string
	CategoryChannels map[string][]models.TvChannel
	TotalChannels    int
}

func NewCatalogGenerator() *CatalogGenerator {
	return &CatalogGenerator{}
}

func (c *CatalogGenerator) GenerateHTML(playlist *models.Playlist, categories []string, categoryChannels map[string][]models.TvChannel) string {
	data := CatalogData{
		PlaylistName:     playlist.Name,
		Date:             time.Now().Format("January 2, 2006"),
		Categories:       categories,
		CategoryChannels: categoryChannels,
		TotalChannels:    len(playlist.Channels),
	}

	// Sort categories alphabetically
	sort.Strings(data.Categories)

	return c.buildHTML(data)
}

func (c *CatalogGenerator) buildHTML(data CatalogData) string {
	var html strings.Builder

	html.WriteString(c.buildHeader(data))
	html.WriteString(c.buildCSS())
	html.WriteString("</head>\n<body>\n")
	html.WriteString(c.buildNavigation(data))
	html.WriteString(c.buildMainContent(data))
	html.WriteString(c.buildJavaScript())
	html.WriteString("</body>\n</html>")

	return html.String()
}

func (c *CatalogGenerator) buildHeader(data CatalogData) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IPTV Catalog - %s</title>
    <meta name="description" content="IPTV Channel Catalog for %s - Generated on %s">
`, html.EscapeString(data.PlaylistName), html.EscapeString(data.PlaylistName), html.EscapeString(data.Date))
}

func (c *CatalogGenerator) buildCSS() string {
	return `    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: #f5f5f5;
            background: linear-gradient(135deg, #0f2027 0%, #2c5364 100%);
            min-height: 100vh;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 0 20px;
        }

        header {
            background: rgba(20, 30, 40, 0.95);
            backdrop-filter: blur(10px);
            padding: 2rem 0;
            margin-bottom: 2rem;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
        }

        h1 {
            font-size: 2.5rem;
            font-weight: 700;
            text-align: center;
            margin-bottom: 0.5rem;
            background: linear-gradient(135deg, #0f2027, #2c5364);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .subtitle {
            text-align: center;
            font-size: 1.2rem;
            color: #bbb;
            margin-bottom: 1rem;
        }

        .stats {
            text-align: center;
            font-size: 1rem;
            color: #999;
        }

        .search-container {
            margin: 2rem 0;
            text-align: center;
        }

        .search-box {
            width: 100%;
            max-width: 500px;
            padding: 12px 20px;
            font-size: 1rem;
            border: 2px solid #444;
            background: #1c1c1c;
            color: #f5f5f5;
            border-radius: 25px;
            outline: none;
            transition: border-color 0.3s ease;
        }

        .search-box:focus {
            border-color: #2c5364;
        }

        .categories-nav {
            background: rgba(20, 30, 40, 0.95);
            backdrop-filter: blur(10px);
            padding: 1.5rem;
            margin-bottom: 2rem;
            border-radius: 15px;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
            position: relative;
        }

        .categories-nav::after {
            content: '';
            position: absolute;
            bottom: 1.5rem;
            left: 1.5rem;
            right: 1.5rem;
            height: 30px;
            background: linear-gradient(transparent, rgba(20, 30, 40, 0.95));
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.3s ease;
        }

        .categories-nav.collapsed::after {
            opacity: 1;
        }

        .categories-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 1rem;
            margin-top: 1rem;
            overflow: hidden;
            transition: max-height 0.4s ease, opacity 0.3s ease;
        }

        .categories-grid.collapsed {
            max-height: calc(3 * (52px + 1rem) + 52px);
        }

        .categories-grid.expanded {
            max-height: 2000px;
        }

        .category-btn {
            background: linear-gradient(135deg, #0f2027, #2c5364);
            color: white;
            border: none;
            padding: 12px 20px;
            border-radius: 8px;
            cursor: pointer;
            font-size: 1rem;
            font-weight: 600;
            transition: all 0.3s ease;
            text-decoration: none;
            display: block;
            text-align: center;
        }

        .category-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(44, 83, 100, 0.5);
        }

        .category-btn.active {
            background: linear-gradient(135deg, #2c5364, #0f2027);
            box-shadow: 0 6px 20px rgba(44, 83, 100, 0.5);
        }

        .show-all-btn {
            background: linear-gradient(135deg, #2c5364, #0f2027);
            grid-column: 1 / -1;
            margin-bottom: 1rem;
            position: relative;
        }

        .show-all-btn:hover {
            box-shadow: 0 6px 20px rgba(44, 83, 100, 0.5);
        }

        .expand-icon {
            margin-left: 0.5rem;
            transition: transform 0.3s ease;
        }

        .show-all-btn.expanded .expand-icon {
            transform: rotate(180deg);
        }

        .category-section {
            background: rgba(20, 30, 40, 0.95);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 2rem;
            margin-bottom: 2rem;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
        }

        .category-section.hidden {
            display: none;
        }

        .category-title {
            font-size: 2rem;
            font-weight: 700;
            margin-bottom: 1.5rem;
            padding-bottom: 0.5rem;
            border-bottom: 3px solid #2c5364;
            color: #f5f5f5;
        }

        .channels-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: 1.5rem;
        }

        .channel-card {
            background: #1f1f1f;
            border-radius: 12px;
            padding: 1.5rem;
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.3);
            transition: all 0.3s ease;
            border: 1px solid #333;
            position: relative;
            overflow: hidden;
        }

        .channel-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
        }

        .channel-logo {
            width: 60px;
            height: 60px;
            object-fit: contain;
            border-radius: 8px;
            margin-bottom: 1rem;
            background: #2c2c2c;
            padding: 8px;
        }

        .channel-name {
            font-size: 1.1rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
            color: #f5f5f5;
            line-height: 1.4;
        }

        .channel-id {
            color: #2c5364;
            font-weight: 600;
            font-size: 0.9rem;
            background: rgba(44, 83, 100, 0.1);
            padding: 4px 8px;
            border-radius: 4px;
            display: inline-block;
        }

        .no-logo {
            width: 60px;
            height: 60px;
            background: linear-gradient(135deg, #0f2027, #2c5364);
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-weight: 700;
            font-size: 1.5rem;
            margin-bottom: 1rem;
        }

        .no-results {
            text-align: center;
            padding: 3rem;
            color: #aaa;
            font-size: 1.2rem;
        }

        .footer {
            text-align: center;
            padding: 2rem;
            color: rgba(255, 255, 255, 0.6);
            font-size: 0.9rem;
        }

        @media (max-width: 768px) {
            .container {
                padding: 0 15px;
            }

            h1 {
                font-size: 2rem;
            }

            .categories-grid {
                grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
                gap: 0.8rem;
            }

            .categories-grid.collapsed {
                max-height: calc(3 * (44px + 0.8rem) + 44px);
            }

            .category-btn {
                padding: 10px 15px;
                font-size: 0.9rem;
            }

            .channels-grid {
                grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
                gap: 1rem;
            }

            .channel-card {
                padding: 1.2rem;
            }
        }

        @media (max-width: 480px) {
            .channels-grid {
                grid-template-columns: 1fr;
            }

            .categories-grid {
                grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
            }

            .categories-grid.collapsed {
                max-height: calc(3 * (40px + 0.8rem) + 40px);
            }
        }
    </style>`
}

func (c *CatalogGenerator) buildNavigation(data CatalogData) string {
	var nav strings.Builder

	nav.WriteString(fmt.Sprintf(`    <header>
        <div class="container">
            <h1>IPTV Channel Catalog</h1>
            <div class="subtitle">%s</div>
            <div class="stats">%d categories • %d channels • Generated on %s</div>
        </div>
    </header>

    <div class="container">
        <div class="search-container">
            <input type="text" id="searchBox" class="search-box" placeholder="Search channels...">
        </div>        <nav class="categories-nav collapsed" id="categoriesNav">
            <div class="categories-grid collapsed" id="categoriesGrid">
                <button class="category-btn show-all-btn active" onclick="toggleCategoriesView()" id="showAllBtn">
                    Show All Categories (%d) <span class="expand-icon">▼</span>
                </button>
`, html.EscapeString(data.PlaylistName), len(data.Categories), data.TotalChannels, html.EscapeString(data.Date), len(data.Categories)))
	for _, category := range data.Categories {
		channelCount := len(data.CategoryChannels[category])
		nav.WriteString(fmt.Sprintf(`                <button class="category-btn" onclick="showCategory('%s')">
                    %s (%d)
                </button>
`, html.EscapeString(category), html.EscapeString(category), channelCount))
	}

	nav.WriteString(`            </div>
        </nav>
`)

	return nav.String()
}

func (c *CatalogGenerator) buildMainContent(data CatalogData) string {
	var content strings.Builder

	content.WriteString(`        <main id="mainContent">
`)

	// Build each category section
	for _, category := range data.Categories {
		channels := data.CategoryChannels[category]
		if len(channels) == 0 {
			continue
		}

		// Sort channels by name within each category
		sort.Slice(channels, func(i, j int) bool {
			return channels[i].Name < channels[j].Name
		})

		content.WriteString(fmt.Sprintf(`            <section class="category-section" data-category="%s">
                <h2 class="category-title">%s <span style="font-weight: 400; color: #666;">(%d channels)</span></h2>
                <div class="channels-grid">
`, html.EscapeString(category), html.EscapeString(category), len(channels)))

		for _, channel := range channels {
			content.WriteString(c.buildChannelCard(channel))
		}

		content.WriteString(`                </div>
            </section>
`)
	}

	content.WriteString(`            <div id="noResults" class="no-results" style="display: none;">
                <h3>No channels found</h3>
                <p>Try adjusting your search terms or selecting a different category.</p>
            </div>
        </main>

        <footer class="footer">
            <div class="container">
                Generated by Discord IPTV Player • ` + time.Now().Format("January 2, 2006 at 3:04 PM") + `
            </div>
        </footer>

    </div>
`)

	return content.String()
}

func (c *CatalogGenerator) buildChannelCard(channel models.TvChannel) string {
	escapedName := html.EscapeString(channel.Name)
	escapedID := html.EscapeString(channel.ID)

	var logoHTML string
	if channel.Logo != "" {
		escapedLogo := html.EscapeString(channel.Logo)
		logoHTML = fmt.Sprintf(`<img src="%s" alt="%s Logo" class="channel-logo" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';">
                        <div class="no-logo" style="display: none;">%s</div>`,
			escapedLogo, escapedName, string([]rune(escapedName)[0:1]))
	} else {
		logoHTML = fmt.Sprintf(`<div class="no-logo">%s</div>`, string([]rune(escapedName)[0:1]))
	}

	return fmt.Sprintf(`                    <div class="channel-card" data-channel-name="%s">
                        %s
                        <h3 class="channel-name">%s</h3>
                        <span class="channel-id">#%s</span>
                    </div>
`, strings.ToLower(escapedName), logoHTML, escapedName, escapedID)
}

func (c *CatalogGenerator) buildJavaScript() string {
	return `    <script>
        let currentCategory = 'all';
        let searchTerm = '';
        let categoriesExpanded = false;        function toggleCategoriesView() {
            const grid = document.getElementById('categoriesGrid');
            const nav = document.getElementById('categoriesNav');
            const btn = document.getElementById('showAllBtn');

            categoriesExpanded = !categoriesExpanded;

            if (categoriesExpanded) {
                grid.classList.remove('collapsed');
                grid.classList.add('expanded');
                nav.classList.remove('collapsed');
                btn.classList.add('expanded');
                btn.innerHTML = btn.innerHTML.replace('▼', '▲');
                btn.innerHTML = btn.innerHTML.replace('Show All Categories', 'Collapse Categories');
            } else {
                grid.classList.remove('expanded');
                grid.classList.add('collapsed');
                nav.classList.add('collapsed');
                btn.classList.remove('expanded');
                btn.innerHTML = btn.innerHTML.replace('▲', '▼');
                btn.innerHTML = btn.innerHTML.replace('Collapse Categories', 'Show All Categories');
            }

            // If categories are collapsed, show all categories content
            if (categoriesExpanded || currentCategory === 'all') {
                showAllCategories();
            }
        }

        function showAllCategories() {
            currentCategory = 'all';
            updateView();
            updateActiveButton('show-all');
        }

        function showCategory(category) {
            currentCategory = category;
            updateView();
            updateActiveButton(category);

            // If a specific category is selected, collapse the categories grid
            if (categoriesExpanded) {
                toggleCategoriesView();
            }
        }

        function updateActiveButton(activeCategory) {
            // Remove active class from all buttons
            document.querySelectorAll('.category-btn').forEach(btn => {
                btn.classList.remove('active');
            });

            // Add active class to the selected button
            if (activeCategory === 'show-all') {
                document.querySelector('.show-all-btn').classList.add('active');
            } else {
                document.querySelectorAll('.category-btn').forEach(btn => {
                    if (btn.textContent.includes(activeCategory)) {
                        btn.classList.add('active');
                    }
                });
            }
        }

        function updateView() {
            const sections = document.querySelectorAll('.category-section');
            const noResults = document.getElementById('noResults');
            let hasVisibleResults = false;

            sections.forEach(section => {
                const category = section.dataset.category;
                const shouldShowCategory = currentCategory === 'all' || currentCategory === category;

                if (shouldShowCategory) {
                    section.style.display = 'block';

                    // Filter channels within this category based on search
                    const channels = section.querySelectorAll('.channel-card');
                    let hasVisibleChannels = false;

                    channels.forEach(channel => {
                        const channelName = channel.dataset.channelName;
                        const matchesSearch = searchTerm === '' || channelName.includes(searchTerm.toLowerCase());

                        if (matchesSearch) {
                            channel.style.display = 'block';
                            hasVisibleChannels = true;
                            hasVisibleResults = true;
                        } else {
                            channel.style.display = 'none';
                        }
                    });

                    // Hide section if no channels match search
                    if (!hasVisibleChannels && searchTerm !== '') {
                        section.style.display = 'none';
                    }
                } else {
                    section.style.display = 'none';
                }
            });

            // Show/hide no results message
            noResults.style.display = hasVisibleResults ? 'none' : 'block';
        }

        // Search functionality
        document.getElementById('searchBox').addEventListener('input', function(e) {
            searchTerm = e.target.value;
            updateView();
        });

        // Smooth scrolling for category buttons
        document.querySelectorAll('.category-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                setTimeout(() => {
                    document.getElementById('mainContent').scrollIntoView({
                        behavior: 'smooth',
                        block: 'start'
                    });
                }, 100);
            });
        });

        // Initialize view
        updateView();
    </script>`
}
