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
	return `    <style>        :root {
            /* Color Palette */
            --primary-bg: #0f2027;
            --secondary-bg: #2c5364;
            --surface-bg: rgba(20, 30, 40, 0.95);
            --card-bg: #2a3441;
            --input-bg: #1c1c1c;
            --border-color: #444;
            --border-light: #3a4a57;            /* Text Colors */
            --text-primary: #f5f5f5;
            --text-secondary: #bbb;
            --text-muted: #999;
            --text-accent: #6b8499;

            /* Interactive Colors */
            --accent-primary: rgba(44, 83, 100, 0.1);
            --accent-hover: rgba(44, 83, 100, 0.2);
            --success-color: #27ae60;
            --success-bg: rgba(46, 204, 113, 0.3);

            /* Shadows */
            --shadow-light: 0 4px 15px rgba(0, 0, 0, 0.3);
            --shadow-medium: 0 4px 20px rgba(0, 0, 0, 0.5);
            --shadow-heavy: 0 8px 30px rgba(0, 0, 0, 0.5);

            /* Transitions */
            --transition-fast: 0.2s ease;
            --transition-normal: 0.3s ease;
            --transition-slow: 0.4s ease;

            /* Border Radius */
            --radius-small: 4px;
            --radius-medium: 8px;
            --radius-large: 12px;
            --radius-xl: 15px;
            --radius-pill: 25px;

            /* Spacing */
            --spacing-xs: 0.25rem;
            --spacing-sm: 0.5rem;
            --spacing-md: 1rem;
            --spacing-lg: 1.5rem;
            --spacing-xl: 2rem;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        *:focus {
            outline: 2px solid var(--secondary-bg);
            outline-offset: 2px;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            color: var(--text-primary);
            background: linear-gradient(135deg, var(--primary-bg) 0%, var(--secondary-bg) 50%, var(--primary-bg) 100%);
            background-attachment: fixed;
            min-height: 100vh;
            -webkit-font-smoothing: antialiased;
            -moz-osx-font-smoothing: grayscale;
        }        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 0 var(--spacing-xl);
        }

        header {
            background: var(--surface-bg);
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
            padding: var(--spacing-xl) 0;
            margin-bottom: var(--spacing-xl);
            box-shadow: var(--shadow-medium);
            border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }

        h1 {
            font-size: clamp(2rem, 5vw, 2.5rem);
            font-weight: 700;
            text-align: center;
            margin-bottom: var(--spacing-sm);
            background: linear-gradient(135deg, var(--primary-bg), var(--secondary-bg), var(--primary-bg));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            background-size: 200% 200%;
            animation: gradientShift 8s ease-in-out infinite;
        }

        @keyframes gradientShift {
            0%, 100% { background-position: 0% 50%; }
            50% { background-position: 100% 50%; }
        }

        .subtitle {
            text-align: center;
            font-size: 1.2rem;
            color: var(--text-secondary);
            margin-bottom: var(--spacing-md);
            font-weight: 500;
        }

        .stats {
            text-align: center;
            font-size: 1rem;
            color: var(--text-muted);
            font-weight: 400;
        }        .search-container {
            margin: var(--spacing-xl) 0;
            text-align: center;
            width: 100%;
            display: flex;
            justify-content: center;
        }

        .search-wrapper {
            position: relative;
            display: inline-block;
            width: 100%;
            max-width: 500px;
        }

        .search-box {
            width: 100%;
            padding: 12px 45px 12px var(--spacing-xl);
            font-size: 1rem;
            border: 2px solid var(--border-color);
            background: var(--input-bg);
            color: var(--text-primary);
            border-radius: var(--radius-pill);
            outline: none;
            transition: all var(--transition-normal);
            font-weight: 500;
            box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.1);
        }

        .search-box:focus {
            border-color: var(--secondary-bg);
            box-shadow: 0 0 0 3px rgba(44, 83, 100, 0.1), inset 0 2px 4px rgba(0, 0, 0, 0.1);
            transform: translateY(-1px);
        }

        .search-box::placeholder {
            color: var(--text-muted);
            font-weight: 400;
        }

        .clear-search {
            position: absolute;
            right: 15px;
            top: 50%;
            transform: translateY(-50%);
            background: none;
            border: none;
            color: var(--text-muted);
            font-size: 1.2rem;
            cursor: pointer;
            padding: 5px;
            border-radius: 50%;
            transition: all var(--transition-normal);
            display: none;
            width: 25px;
            height: 25px;
            align-items: center;
            justify-content: center;
            will-change: transform;
        }

        .clear-search:hover {
            background: var(--accent-hover);
            color: var(--text-primary);
            transform: translateY(-50%) scale(1.1);
        }

        .clear-search:active {
            transform: translateY(-50%) scale(0.95);
        }

        .clear-search.visible {
            display: flex;
        }        .categories-nav {
            background: var(--surface-bg);
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
            padding: var(--spacing-lg);
            margin-bottom: var(--spacing-xl);
            border-radius: var(--radius-xl);
            box-shadow: var(--shadow-medium);
            position: relative;
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .categories-nav::after {
            content: '';
            position: absolute;
            bottom: var(--spacing-lg);
            left: var(--spacing-lg);
            right: var(--spacing-lg);
            height: 30px;
            background: linear-gradient(transparent, var(--surface-bg));
            pointer-events: none;
            opacity: 0;
            transition: opacity var(--transition-normal);
        }

        .categories-nav.collapsed::after {
            opacity: 1;
        }

        .categories-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: var(--spacing-md);
            margin-top: var(--spacing-md);
            overflow: hidden;
            transition: max-height var(--transition-slow), opacity var(--transition-normal);
        }

        .categories-grid.collapsed {
            max-height: calc(3 * (52px + var(--spacing-md)) + 52px);
        }

        .categories-grid.expanded {
            max-height: 2000px;
        }        .category-btn {
            background: var(--card-bg);
            color: var(--text-primary);
            border: 1px solid var(--border-light);
            padding: var(--spacing-lg);
            border-radius: var(--radius-large);
            cursor: pointer;
            font-size: 1rem;
            font-weight: 600;
            transition: all var(--transition-normal);
            text-decoration: none;
            display: flex;
            align-items: center;
            justify-content: center;
            text-align: center;
            position: relative;
            overflow: visible;
            will-change: transform;
            box-shadow: var(--shadow-light);
            min-height: 60px;
        }

        .category-btn::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: linear-gradient(135deg, transparent, rgba(44, 83, 100, 0.05));
            opacity: 0;
            transition: opacity var(--transition-normal);
            border-radius: var(--radius-large);
            pointer-events: none;
        }

        .category-btn:hover::before {
            opacity: 1;
        }

        .category-btn:hover {
            transform: translateY(-5px);
            box-shadow: var(--shadow-heavy);
            border-color: rgba(44, 83, 100, 0.3);
        }

        .category-btn:active {
            transform: translateY(-2px);
        }

        .category-btn.active {
            background: var(--card-bg);
            border-color: var(--secondary-bg);
            box-shadow: var(--shadow-heavy);
            color: var(--text-primary);
        }

        .category-btn.active::before {
            background: linear-gradient(135deg, rgba(44, 83, 100, 0.1), rgba(44, 83, 100, 0.05));
            opacity: 1;
        }        .show-all-btn {
            background: var(--card-bg);
            border: 1px solid var(--border-light);
            grid-column: 1 / -1;
            margin-bottom: var(--spacing-md);
            position: relative;
            color: var(--text-primary);
            box-shadow: var(--shadow-light);
        }

        .show-all-btn:hover {
            box-shadow: var(--shadow-heavy);
            border-color: rgba(44, 83, 100, 0.3);
        }

        .show-all-btn.active {
            border-color: var(--secondary-bg);
            box-shadow: var(--shadow-heavy);
        }

        .show-all-btn.active::before {
            background: linear-gradient(135deg, rgba(44, 83, 100, 0.1), rgba(44, 83, 100, 0.05));
            opacity: 1;
        }

        .expand-icon {
            margin-left: var(--spacing-sm);
            transition: transform var(--transition-normal);
            display: inline-block;
        }

        .show-all-btn.expanded .expand-icon {
            transform: rotate(180deg);
        }

        .category-section {
            background: var(--surface-bg);
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
            border-radius: var(--radius-xl);
            padding: var(--spacing-xl);
            margin-bottom: var(--spacing-xl);
            box-shadow: var(--shadow-medium);
            border: 1px solid rgba(255, 255, 255, 0.1);
            will-change: transform;
        }

        .category-section.hidden {
            display: none;
        }

        .category-title {
            font-size: clamp(1.5rem, 4vw, 2rem);
            font-weight: 700;
            margin-bottom: var(--spacing-lg);
            padding-bottom: var(--spacing-sm);
            border-bottom: 3px solid var(--secondary-bg);
            color: var(--text-primary);
            position: relative;
        }

        .category-title::after {
            content: '';
            position: absolute;
            bottom: -3px;
            left: 0;
            width: 50px;
            height: 3px;
            background: linear-gradient(90deg, var(--secondary-bg), transparent);
        }        .channels-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
            gap: var(--spacing-lg);
        }

        /* Compact channel card - 'c' class for minimal HTML */
        .c {
            background: var(--card-bg);
            border-radius: var(--radius-large);
            padding: var(--spacing-lg);
            box-shadow: var(--shadow-light);
            transition: all var(--transition-normal);
            border: 1px solid var(--border-light);
            position: relative;
            overflow: visible;
            will-change: transform;
            min-height: 120px;
        }

        .c::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: linear-gradient(135deg, transparent, rgba(44, 83, 100, 0.05));
            opacity: 0;
            transition: opacity var(--transition-normal);
            border-radius: var(--radius-large);
            pointer-events: none;
        }

        .c:hover::before {
            opacity: 1;
        }

        .c:hover {
            transform: translateY(-5px);
            box-shadow: var(--shadow-heavy);
            border-color: rgba(44, 83, 100, 0.3);
        }

        /* Legacy styles for backward compatibility */
        .channel-card {
            background: var(--card-bg);
            border-radius: var(--radius-large);
            padding: var(--spacing-lg);
            box-shadow: var(--shadow-light);
            transition: all var(--transition-normal);
            border: 1px solid var(--border-light);
            position: relative;
            overflow: visible;
            will-change: transform;
        }

        .channel-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: linear-gradient(135deg, transparent, rgba(44, 83, 100, 0.05));
            opacity: 0;
            transition: opacity var(--transition-normal);
            border-radius: var(--radius-large);
            pointer-events: none;
        }

        .channel-card:hover::before {
            opacity: 1;
        }

        .channel-card:hover {
            transform: translateY(-5px);
            box-shadow: var(--shadow-heavy);
            border-color: rgba(44, 83, 100, 0.3);
        }

        .channel-logo {
            width: 60px;
            height: 60px;
            object-fit: contain;
            border-radius: var(--radius-medium);
            margin-bottom: var(--spacing-md);
            background: #2c2c2c;
            padding: var(--spacing-sm);
            transition: transform var(--transition-fast);
        }

        .channel-card:hover .channel-logo {
            transform: scale(1.05);
        }

        .channel-name {
            font-size: 1.1rem;
            font-weight: 600;
            margin-bottom: var(--spacing-sm);
            color: var(--text-primary);
            line-height: 1.4;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }        .channel-id {
            color: var(--text-accent);
            font-weight: 600;
            font-size: 0.9rem;
            background: var(--accent-primary);
            padding: var(--spacing-xs) var(--spacing-sm);
            border-radius: var(--radius-small);
            display: inline-flex;
            align-items: center;
            cursor: pointer;
            transition: all var(--transition-normal);
            user-select: none;
            position: relative;
            border: 1px solid transparent;
            will-change: transform;
        }

        .channel-id:hover {
            background: var(--accent-hover);
            transform: translateY(-1px);
            border-color: rgba(44, 83, 100, 0.3);
        }

        .channel-id:active {
            transform: translateY(0);
        }

        .channel-id.copied {
            animation: blink 0.6s ease-in-out;
        }

        @keyframes blink {
            0%, 100% {
                background: var(--accent-primary);
                color: var(--text-accent);
            }
            50% {
                background: var(--success-bg);
                color: var(--success-color);
                box-shadow: 0 0 10px var(--success-bg);
            }
        }

        .copy-icon {
            margin-left: var(--spacing-sm);
            font-size: 0.8rem;
            opacity: 0.7;
            transition: all var(--transition-fast);
        }

        .channel-id:hover .copy-icon {
            opacity: 1;
            transform: scale(1.1);
        }        .copy-feedback {
            position: absolute;
            top: -35px;
            left: 50%;
            transform: translateX(-50%);
            background: var(--surface-bg);
            color: #e0e6ed;
            padding: 6px 12px;
            border-radius: 6px;
            font-size: 0.8rem;
            font-weight: 600;
            white-space: nowrap;
            opacity: 0;
            pointer-events: none;
            transition: all var(--transition-normal);
            box-shadow: var(--shadow-light);
            border: 1px solid rgba(96, 165, 250, 0.3);
            z-index: 1000;
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
        }

        .copy-feedback::after {
            content: '';
            position: absolute;
            top: 100%;
            left: 50%;
            transform: translateX(-50%);
            border: 5px solid transparent;
            border-top-color: var(--surface-bg);
        }

        .copy-feedback.show {
            opacity: 1;
            transform: translateX(-50%) translateY(-5px);
            animation: fadeInOut 0.5s ease-in-out;
        }

        @keyframes fadeInOut {
            0% {
                opacity: 0;
                transform: translateX(-50%) translateY(0px) scale(0.8);
            }
            30% {
                opacity: 1;
                transform: translateX(-50%) translateY(-5px) scale(1);
            }
            70% {
                opacity: 1;
                transform: translateX(-50%) translateY(-5px) scale(1);
            }
            100% {
                opacity: 0;
                transform: translateX(-50%) translateY(-10px) scale(0.9);
            }
        }

        .no-logo {
            width: 60px;
            height: 60px;
            background: linear-gradient(135deg, var(--primary-bg), var(--secondary-bg));
            border-radius: var(--radius-medium);
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-weight: 700;
            font-size: 1.5rem;
            margin-bottom: var(--spacing-md);
            transition: transform var(--transition-fast);
        }

        .channel-card:hover .no-logo {
            transform: scale(1.05);
        }

        .no-results {
            text-align: center;
            padding: 3rem;
            color: var(--text-secondary);
            font-size: 1.2rem;
        }

        .no-results h3 {
            margin-bottom: var(--spacing-md);
            color: var(--text-primary);
        }

        .footer {
            text-align: center;
            padding: var(--spacing-xl);
            color: rgba(255, 255, 255, 0.6);
            font-size: 0.9rem;
            border-top: 1px solid rgba(255, 255, 255, 0.1);
            background: var(--surface-bg);
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
        }@media (max-width: 768px) {
            .container {
                padding: 0 15px;
            }

            h1 {
                font-size: 2rem;
            }

            .search-wrapper {
                max-width: 100%;
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
    </header>    <div class="container">
        <div class="search-container">
            <div class="search-wrapper">
                <input type="text" id="searchBox" class="search-box" placeholder="Search channels...">
                <button class="clear-search" id="clearSearch" onclick="clearSearch()" title="Clear search">✕</button>
            </div>
        </div><nav class="categories-nav collapsed" id="categoriesNav">
            <div class="categories-grid collapsed" id="categoriesGrid">                <button class="category-btn show-all-btn active" onclick="toggleCategoriesView()" id="showAllBtn">
                    Show All Categories (%d) <span class="expand-icon">▼</span>
                </button>
`, html.EscapeString(data.PlaylistName), len(data.Categories), data.TotalChannels, html.EscapeString(data.Date), len(data.Categories)))
	for _, category := range data.Categories {
		channelCount := len(data.CategoryChannels[category])
		nav.WriteString(fmt.Sprintf(`                <button class="category-btn" onclick="toggleCategory('%s')" data-category="%s">
                    %s (%d)
                </button>
`, html.EscapeString(category), html.EscapeString(category), html.EscapeString(category), channelCount))
	}

	nav.WriteString(`            </div>
        </nav>
`)

	return nav.String()
}

func (c *CatalogGenerator) buildMainContent(data CatalogData) string {
	var content strings.Builder
	content.WriteString(`<main id="mainContent">`)

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
		content.WriteString(fmt.Sprintf(`<section class="category-section" data-category="%s"><h2 class="category-title">%s <span style="font-weight:400;color:#666;">(%d channels)</span></h2><div class="channels-grid">`, html.EscapeString(category), html.EscapeString(category), len(channels)))

		for _, channel := range channels {
			content.WriteString(c.buildChannelCard(channel))
		}
		content.WriteString(`</div></section>`)
	}
	content.WriteString(`<div id="noResults" class="no-results" style="display:none;"><h3>No channels found</h3><p>Try adjusting your search terms or selecting a different category.</p></div></main><footer class="footer"><div class="container">Generated by Discord IPTV Player • ` + time.Now().Format("January 2, 2006 at 3:04 PM") + `</div></footer></div>`)

	return content.String()
}

func (c *CatalogGenerator) buildChannelCard(channel models.TvChannel) string {
	escapedName := html.EscapeString(channel.Name)
	escapedID := html.EscapeString(channel.ID)

	// Use minimal HTML structure - JavaScript will handle the rest
	logoAttr := ""
	if channel.Logo != "" {
		logoAttr = fmt.Sprintf(` data-logo="%s"`, html.EscapeString(channel.Logo))
	}

	// Ultra-compact structure: ~80% size reduction
	return fmt.Sprintf(`<div class="c" data-n="%s" data-i="%s"%s></div>`,
		strings.ToLower(escapedName), escapedID, logoAttr)
}

func (c *CatalogGenerator) buildJavaScript() string {
	return `    <script>
        // Application state
        const AppState = {
            currentCategory: 'all',
            searchTerm: '',
            categoriesExpanded: false,
            searchTimeout: null
        };

        // DOM element cache for better performance
        const DOMCache = {
            searchBox: null,
            clearBtn: null,
            categoriesGrid: null,
            categoriesNav: null,
            showAllBtn: null,
            noResults: null,
            sections: null,
            categoryBtns: null,
            mainContent: null,

            init() {
                this.searchBox = document.getElementById('searchBox');
                this.clearBtn = document.getElementById('clearSearch');
                this.categoriesGrid = document.getElementById('categoriesGrid');
                this.categoriesNav = document.getElementById('categoriesNav');
                this.showAllBtn = document.getElementById('showAllBtn');
                this.noResults = document.getElementById('noResults');
                this.mainContent = document.getElementById('mainContent');

                // Cache NodeLists
                this.sections = document.querySelectorAll('.category-section');
                this.categoryBtns = document.querySelectorAll('.category-btn');
            },

            refresh() {
                // Refresh dynamic elements if needed
                this.sections = document.querySelectorAll('.category-section');
                this.categoryBtns = document.querySelectorAll('.category-btn');
            }
        };

        // Utility functions
        const Utils = {
            debounce(func, wait) {
                return function executedFunction(...args) {
                    const later = () => {
                        clearTimeout(AppState.searchTimeout);
                        func(...args);
                    };
                    clearTimeout(AppState.searchTimeout);
                    AppState.searchTimeout = setTimeout(later, wait);
                };
            },

            sanitizeInput(input) {
                return input.toLowerCase().trim();
            },

            showElement(element, display = 'block') {
                if (element) element.style.display = display;
            },

            hideElement(element) {
                if (element) element.style.display = 'none';
            },

            scrollToElement(element, options = { behavior: 'smooth', block: 'start' }) {
                if (element) {
                    element.scrollIntoView(options);
                }
            }
        };

        // Search functionality
        const SearchManager = {
            clear() {
                if (!DOMCache.searchBox || !DOMCache.clearBtn) return;

                DOMCache.searchBox.value = '';
                AppState.searchTerm = '';
                DOMCache.clearBtn.classList.remove('visible');
                ViewManager.update();
                DOMCache.searchBox.focus();
            },

            toggleClearButton() {
                if (!DOMCache.searchBox || !DOMCache.clearBtn) return;

                if (DOMCache.searchBox.value.length > 0) {
                    DOMCache.clearBtn.classList.add('visible');
                } else {
                    DOMCache.clearBtn.classList.remove('visible');
                }
            },

            handleInput: Utils.debounce(function(value) {
                AppState.searchTerm = Utils.sanitizeInput(value);
                SearchManager.toggleClearButton();
                ViewManager.update();
            }, 150)
        };

        // Clipboard functionality
        const ClipboardManager = {
            async copy(channelId, event) {
                if (!event) return;

                const clickedElement = event.target.closest('.channel-id');
                const feedback = clickedElement?.querySelector('.copy-feedback');

                if (!clickedElement || !feedback) return;

                try {
                    if (navigator.clipboard && window.isSecureContext) {
                        await navigator.clipboard.writeText(channelId);
                    } else {
                        // Fallback for older browsers or non-secure contexts
                        this.fallbackCopy(channelId);
                    }
                    this.showFeedback(clickedElement, feedback);
                } catch (err) {
                    console.warn('Clipboard copy failed, using fallback:', err);
                    this.fallbackCopy(channelId);
                    this.showFeedback(clickedElement, feedback);
                }
            },

            fallbackCopy(text) {
                const textArea = document.createElement('textarea');
                textArea.value = text;
                textArea.style.cssText = 'position:fixed;left:-999999px;top:-999999px;opacity:0;';
                document.body.appendChild(textArea);
                textArea.focus();
                textArea.select();

                try {
                    document.execCommand('copy');
                } catch (err) {
                    console.error('Fallback copy failed:', err);
                } finally {
                    document.body.removeChild(textArea);
                }
            },

            showFeedback(element, feedback) {
                if (!element || !feedback) return;

                // Add blinking effect to the channel ID
                element.classList.add('copied');
                feedback.classList.add('show');

                // Clean up animations
                setTimeout(() => element.classList.remove('copied'), 600);
                setTimeout(() => feedback.classList.remove('show'), 2000);
            }
        };

        // Category management
        const CategoryManager = {
            toggleView() {
                if (!DOMCache.categoriesGrid || !DOMCache.categoriesNav || !DOMCache.showAllBtn) return;

                AppState.categoriesExpanded = !AppState.categoriesExpanded;

                if (AppState.categoriesExpanded) {
                    this.expandCategories();
                } else {
                    this.collapseCategories();
                }

                // If categories are expanded or showing all, show all categories content
                if (AppState.categoriesExpanded || AppState.currentCategory === 'all') {
                    this.showAll();
                }
            },

            expandCategories() {
                DOMCache.categoriesGrid.classList.remove('collapsed');
                DOMCache.categoriesGrid.classList.add('expanded');
                DOMCache.categoriesNav.classList.remove('collapsed');
                DOMCache.showAllBtn.classList.add('expanded');
                this.updateButtonText('Collapse Categories', '▲');
            },

            collapseCategories() {
                DOMCache.categoriesGrid.classList.remove('expanded');
                DOMCache.categoriesGrid.classList.add('collapsed');
                DOMCache.categoriesNav.classList.add('collapsed');
                DOMCache.showAllBtn.classList.remove('expanded');
                this.updateButtonText('Show All Categories', '▼');
            },            updateButtonText(text, arrow) {
                if (!DOMCache.showAllBtn) return;

                const currentHTML = DOMCache.showAllBtn.innerHTML;
                const match = currentHTML.match(/\((\d+)\)/);
                const count = match ? match[1] : '';

                DOMCache.showAllBtn.innerHTML = text + ' (' + count + ') <span class="expand-icon">' + arrow + '</span>';
            },

            showAll() {
                AppState.currentCategory = 'all';
                ViewManager.update();
                this.updateActiveButton('show-all');
            },

            show(category) {
                AppState.currentCategory = category;
                ViewManager.update();
                this.updateActiveButton(category);

                // Auto-collapse if a specific category is selected
                if (AppState.categoriesExpanded) {
                    this.toggleView();
                }
            },

            toggle(category) {
                if (AppState.currentCategory === category) {
                    this.showAll();
                } else {
                    this.show(category);
                }
            },

            updateActiveButton(activeCategory) {
                if (!DOMCache.categoryBtns) return;

                // Remove active class from all buttons
                DOMCache.categoryBtns.forEach(btn => btn.classList.remove('active'));

                // Add active class to the selected button
                if (activeCategory === 'show-all') {
                    DOMCache.showAllBtn?.classList.add('active');
                } else {
                    DOMCache.categoryBtns.forEach(btn => {
                        if (btn.textContent.includes(activeCategory)) {
                            btn.classList.add('active');
                        }
                    });
                }
            }
        };        // View management
        const ViewManager = {
            update() {
                if (!DOMCache.sections || !DOMCache.noResults) return;

                let hasVisibleResults = false;

                DOMCache.sections.forEach(section => {
                    const category = section.dataset.category;
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === category;

                    if (shouldShowCategory) {
                        Utils.showElement(section);
                        const hasVisibleChannels = this.filterChannelsInSection(section);

                        if (!hasVisibleChannels && AppState.searchTerm !== '') {
                            Utils.hideElement(section);
                        } else if (hasVisibleChannels) {
                            hasVisibleResults = true;
                        }
                    } else {
                        Utils.hideElement(section);
                    }
                });

                // Show/hide no results message
                if (hasVisibleResults) {
                    Utils.hideElement(DOMCache.noResults);
                } else {
                    Utils.showElement(DOMCache.noResults);
                }
            },

            filterChannelsInSection(section) {
                const channels = section.querySelectorAll('.c');
                let hasVisibleChannels = false;

                channels.forEach(channel => {
                    const channelName = channel.dataset.n;
                    const matchesSearch = AppState.searchTerm === '' ||
                                        (channelName && channelName.includes(AppState.searchTerm));

                    if (matchesSearch) {
                        Utils.showElement(channel);
                        hasVisibleChannels = true;
                    } else {
                        Utils.hideElement(channel);
                    }
                });

                return hasVisibleChannels;
            }
        };        // Dynamic content generator for compact cards
        const ContentGenerator = {
            generateChannelCards() {
                document.querySelectorAll('.c').forEach(card => {
                    if (card.innerHTML) return; // Already generated

                    const name = card.dataset.n;
                    const id = card.dataset.i;
                    const logo = card.dataset.logo;

                    const displayName = this.capitalize(name);
                    const firstLetter = displayName.charAt(0).toUpperCase();

                    const logoHtml = logo ?
                        '<img src="' + this.escapeHtml(logo) + '" alt="' + this.escapeHtml(displayName) + ' Logo" class="channel-logo" onerror="this.style.display=\'none\';this.nextElementSibling.style.display=\'flex\';"><div class="no-logo" style="display:none;">' + firstLetter + '</div>' :
                        '<div class="no-logo">' + firstLetter + '</div>';

                    card.innerHTML = logoHtml + '<h3 class="channel-name">' + this.escapeHtml(displayName) + '</h3><span class="channel-id" onclick="copyChannelId(\'' + this.escapeHtml(id) + '\')" title="Click to copy channel ID">#' + this.escapeHtml(id) + ' <span class="copy-icon">📋</span><div class="copy-feedback">Copied!</div></span>';
                });
            },

            capitalize(str) {
                return str.split(' ').map(word =>
                    word.charAt(0).toUpperCase() + word.slice(1).toLowerCase()
                ).join(' ');
            },

            escapeHtml(text) {
                const div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
            }
        };

        // Keyboard navigation
        const KeyboardManager = {
            init() {
                document.addEventListener('keydown', this.handleKeydown.bind(this));
            },

            handleKeydown(event) {
                // ESC key to clear search
                if (event.key === 'Escape' && AppState.searchTerm) {
                    event.preventDefault();
                    SearchManager.clear();
                }

                // Ctrl/Cmd + F to focus search
                if ((event.ctrlKey || event.metaKey) && event.key === 'f') {
                    event.preventDefault();
                    DOMCache.searchBox?.focus();
                }
            }
        };

        // Global functions (needed for inline onclick handlers)
        window.clearSearch = () => SearchManager.clear();
        window.copyChannelId = (channelId) => ClipboardManager.copy(channelId, window.event);
        window.toggleCategoriesView = () => CategoryManager.toggleView();
        window.toggleCategory = (category) => CategoryManager.toggle(category);

        // Event listeners
        function initializeEventListeners() {
            // Search input with debounced handling
            DOMCache.searchBox?.addEventListener('input', (e) => {
                SearchManager.handleInput(e.target.value);
            });

            // Smooth scrolling for category buttons
            DOMCache.categoryBtns?.forEach(btn => {
                btn.addEventListener('click', () => {
                    setTimeout(() => {
                        Utils.scrollToElement(DOMCache.mainContent);
                    }, 100);
                });
            });

            // Initialize keyboard navigation
            KeyboardManager.init();
        }        // Application initialization
        function initializeApp() {
            DOMCache.init();
            ContentGenerator.generateChannelCards(); // Generate compact card content
            SearchManager.toggleClearButton();
            ViewManager.update();
            initializeEventListeners();
        }// Initialize when DOM is ready
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', initializeApp);
        } else {
            initializeApp();
        }
    </script>`
}

// EstimateFileSize returns the estimated file size in bytes and a summary of optimizations
func (c *CatalogGenerator) EstimateFileSize(totalChannels int) (int, string) {
	// Base HTML structure (header, CSS, JavaScript, footer): ~15KB
	baseSize := 15 * 1024

	// Old method: ~280 bytes per channel (full HTML structure)
	// New method: ~45 bytes per channel (minimal data attributes)
	oldChannelSize := 280
	newChannelSize := 45

	oldTotalSize := baseSize + (totalChannels * oldChannelSize)
	newTotalSize := baseSize + (totalChannels * newChannelSize)

	reduction := float64(oldTotalSize-newTotalSize) / float64(oldTotalSize) * 100

	summary := fmt.Sprintf(`File Size Optimization Summary:
- Base structure: ~15KB
- Old method: %d channels × %d bytes = %d KB total (%.1f MB)
- New method: %d channels × %d bytes = %d KB total (%.1f MB)
- Size reduction: %.1f%% (Saved: %.1f MB)
- Techniques used:
  • Minified HTML structure (removed whitespace/indentation)
  • Compact CSS class names (.c instead of .channel-card)
  • Data attributes instead of full HTML content
  • JavaScript-based content generation
  • Removed redundant HTML elements`,
		totalChannels, oldChannelSize, oldTotalSize/1024, float64(oldTotalSize)/(1024*1024),
		totalChannels, newChannelSize, newTotalSize/1024, float64(newTotalSize)/(1024*1024),
		reduction, float64(oldTotalSize-newTotalSize)/(1024*1024))

	return newTotalSize, summary
}
