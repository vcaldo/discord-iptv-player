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

// GenerateHTMLWithEmbeddedJSON creates HTML with embedded JSON data for better performance
func (c *CatalogGenerator) GenerateHTMLWithEmbeddedJSON(jsonData string) string {
	var html strings.Builder

	html.WriteString(c.buildHeaderForEmbeddedJSON())
	html.WriteString(c.buildCSS())
	html.WriteString("</head>\n<body>\n")
	html.WriteString(c.buildNavigationForEmbeddedJSON())
	html.WriteString(c.buildMainContentForEmbeddedJSON())
	html.WriteString(c.buildEmbeddedJSONScript(jsonData))
	html.WriteString(c.buildJavaScriptForEmbeddedJSON())
	html.WriteString("</body>\n</html>")

	return html.String()
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
        }        .subtitle {
            text-align: center;
            font-size: 1.2rem;
            color: var(--text-secondary);
            margin-bottom: var(--spacing-md);
            font-weight: 500;
        }

        .playlist-command {
            text-align: center;
            margin: var(--spacing-sm) 0 var(--spacing-lg) 0;
        }

        .playlist-id {
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

        .playlist-id:hover {
            background: var(--accent-hover);
            transform: translateY(-1px);
            border-color: rgba(44, 83, 100, 0.3);
        }

        .playlist-id:active {
            transform: translateY(0);
        }

        .playlist-id.copied {
            animation: blink 0.6s ease-in-out;
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
        }        .pagination-container {
            display: flex;
            justify-content: center;
            align-items: center;
            margin: var(--spacing-xl) 0;
            gap: var(--spacing-md);
        }

        .pagination-info {
            color: var(--text-secondary);
            font-size: 0.9rem;
            background: var(--surface-bg);
            padding: var(--spacing-sm) var(--spacing-md);
            border-radius: var(--radius-large);
            border: 1px solid rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
        }

        .pagination-controls {
            display: flex;
            gap: var(--spacing-sm);
            align-items: center;
        }

        .pagination-btn {
            background: var(--card-bg);
            color: var(--text-primary);
            border: 1px solid var(--border-light);
            padding: var(--spacing-sm) var(--spacing-md);
            border-radius: var(--radius-medium);
            cursor: pointer;
            font-size: 0.9rem;
            font-weight: 600;
            transition: all var(--transition-normal);
            min-width: 40px;
            text-align: center;
            box-shadow: var(--shadow-light);
            position: relative;
            overflow: hidden;
        }

        .pagination-btn::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: linear-gradient(135deg, transparent, rgba(44, 83, 100, 0.05));
            opacity: 0;
            transition: opacity var(--transition-normal);
            pointer-events: none;
        }

        .pagination-btn:hover::before {
            opacity: 1;
        }

        .pagination-btn:hover {
            transform: translateY(-2px);
            box-shadow: var(--shadow-heavy);
            border-color: rgba(44, 83, 100, 0.3);
        }

        .pagination-btn:active {
            transform: translateY(0);
        }

        .pagination-btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
            transform: none;
            box-shadow: var(--shadow-light);
        }

        .pagination-btn:disabled:hover {
            transform: none;
            box-shadow: var(--shadow-light);
            border-color: var(--border-light);
        }

        .pagination-btn:disabled::before {
            opacity: 0;
        }

        .pagination-btn.active {
            background: var(--secondary-bg);
            border-color: var(--secondary-bg);
            color: var(--text-primary);
            font-weight: 700;
        }

        .pagination-btn.active::before {
            background: linear-gradient(135deg, rgba(44, 83, 100, 0.1), rgba(44, 83, 100, 0.05));
            opacity: 1;
        }

        .page-size-selector {
            display: flex;
            align-items: center;
            gap: var(--spacing-sm);
            color: var(--text-secondary);
            font-size: 0.9rem;
        }

        .page-size-select {
            background: var(--card-bg);
            color: var(--text-primary);
            border: 1px solid var(--border-light);
            padding: var(--spacing-xs) var(--spacing-sm);
            border-radius: var(--radius-medium);
            font-size: 0.9rem;
            cursor: pointer;
            transition: all var(--transition-normal);
        }

        .page-size-select:focus {
            outline: none;
            border-color: var(--secondary-bg);
            box-shadow: 0 0 0 2px rgba(44, 83, 100, 0.1);
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
            <div class="playlist-command">
                <span class="playlist-id" onclick="copyPlaylistId('%s')" title="Click to copy Discord command">/playlist name:%s <span class="copy-icon">📋</span><div class="copy-feedback">Command copied!</div></span>
            </div>
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
`, html.EscapeString(data.PlaylistName), html.EscapeString(data.PlaylistName), html.EscapeString(data.PlaylistName), len(data.Categories), data.TotalChannels, html.EscapeString(data.Date), len(data.Categories)))
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

	// Add global pagination controls before categories
	content.WriteString(`<div class="pagination-container global-pagination">
		<div class="page-size-selector">
			<label>Per page:</label>
			<select class="page-size-select" onchange="updatePageSize(this.value)">
				<option value="50">50</option>
				<option value="100">100</option>
				<option value="200" selected>200</option>
				<option value="500">500</option>
			</select>
		</div>
		<div class="pagination-info">
			<span class="page-info">Page <span class="current-page">1</span> of <span class="total-pages">1</span></span>
			<span class="results-info">(<span class="visible-channels">0</span> of <span class="total-channels">0</span> channels)</span>
		</div>
		<div class="pagination-controls">
			<button class="pagination-btn" onclick="goToFirstPage()" title="First page">⏮</button>
			<button class="pagination-btn" onclick="goToPrevPage()" title="Previous page">◀</button>
			<div class="page-numbers"></div>
			<button class="pagination-btn" onclick="goToNextPage()" title="Next page">▶</button>
			<button class="pagination-btn" onclick="goToLastPage()" title="Last page">⏭</button>
		</div>
	</div>`)

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
		content.WriteString(fmt.Sprintf(`<section class="category-section" data-category="%s"><h2 class="category-title">%s <span style="font-weight:400;color:#666;">(<span class="channel-count">%d</span> channels)</span></h2>`, html.EscapeString(category), html.EscapeString(category), len(channels)))

		content.WriteString(`<div class="channels-grid">`)
		for _, channel := range channels {
			content.WriteString(c.buildChannelCard(channel))
		}
		content.WriteString(`</div>`)
		content.WriteString(`</section>`)
	}

	content.WriteString(`<div id="noResults" class="no-results" style="display:none;"><h3>No channels found</h3><p>Try adjusting your search terms or selecting a different category.</p></div></main>`)
	content.WriteString(`<footer class="footer"><div class="container">Generated by Discord IPTV Player • ` + time.Now().Format("January 2, 2006 at 3:04 PM") + `</div></footer></div>`)

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
	return `    <script>        // Application state
        const AppState = {
            currentCategory: 'all',
            searchTerm: '',
            categoriesExpanded: false,
            searchTimeout: null,            pagination: {
                currentPage: 1,
                pageSize: 200,
                totalPages: 1,
                totalChannels: 0,
                visibleChannels: 0
            }
        };        // Enhanced DOM cache with channel data optimization
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

            // Performance optimization: Cache channel data
            allChannelData: [],
            visibleChannelElements: new Map(),
            sectionChannelMap: new Map(),

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

                // Performance: Pre-cache all channel data
                this.cacheChannelData();
            },

            cacheChannelData() {
                console.time('Channel data caching');
                this.allChannelData = [];
                this.sectionChannelMap.clear();

                this.sections.forEach(section => {
                    const category = section.dataset.category;
                    const channels = Array.from(section.querySelectorAll('.c'));

                    // Cache channels by category
                    this.sectionChannelMap.set(category, channels);

                    // Cache channel data for faster searching
                    channels.forEach(channel => {
                        this.allChannelData.push({
                            element: channel,
                            name: (channel.dataset.n || '').toLowerCase(),
                            category: category,
                            id: channel.dataset.i,
                            logo: channel.dataset.logo
                        });
                    });
                });                console.timeEnd('Channel data caching');
                console.log('Cached ' + this.allChannelData.length + ' channels across ' + this.sections.length + ' categories');
            },

            refresh() {
                // Only refresh if needed - avoid unnecessary DOM queries
                const currentSectionCount = document.querySelectorAll('.category-section').length;
                if (currentSectionCount !== this.sections.length) {
                    this.sections = document.querySelectorAll('.category-section');
                    this.categoryBtns = document.querySelectorAll('.category-btn');
                    this.cacheChannelData();
                }
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
            },            handleInput: Utils.debounce(function(value) {
                AppState.searchTerm = Utils.sanitizeInput(value);
                AppState.pagination.currentPage = 1; // Reset to first page when searching
                SearchManager.toggleClearButton();

                // Use optimized search for large datasets
                if (DOMCache.allChannelData.length > 1000) {
                    ViewManager.updateOptimized();
                } else {
                    ViewManager.update();
                }
            }, 150)
        };        // Clipboard functionality
        const ClipboardManager = {
            async copy(channelId, event) {
                if (!event) return;

                const clickedElement = event.target.closest('.channel-id');
                const feedback = clickedElement?.querySelector('.copy-feedback');                if (!clickedElement || !feedback) return;

                const discordCommand = '/tv channel:' + channelId;

                try {
                    if (navigator.clipboard && window.isSecureContext) {
                        await navigator.clipboard.writeText(discordCommand);
                    } else {
                        // Fallback for older browsers or non-secure contexts
                        this.fallbackCopy(discordCommand);
                    }
                    this.showFeedback(clickedElement, feedback);
                } catch (err) {
                    console.warn('Clipboard copy failed, using fallback:', err);
                    this.fallbackCopy(discordCommand);
                    this.showFeedback(clickedElement, feedback);
                }
            },

            async copyPlaylist(playlistName, event) {
                if (!event) return;

                const clickedElement = event.target.closest('.playlist-id');
                const feedback = clickedElement?.querySelector('.copy-feedback');

                if (!clickedElement || !feedback) return;

                const discordCommand = '/playlist name:' + playlistName;

                try {
                    if (navigator.clipboard && window.isSecureContext) {
                        await navigator.clipboard.writeText(discordCommand);
                    } else {
                        // Fallback for older browsers or non-secure contexts
                        this.fallbackCopy(discordCommand);
                    }
                    this.showFeedback(clickedElement, feedback);
                } catch (err) {
                    console.warn('Clipboard copy failed, using fallback:', err);
                    this.fallbackCopy(discordCommand);
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
            },            showAll() {
                AppState.currentCategory = 'all';
                AppState.pagination.currentPage = 1; // Reset pagination
                ViewManager.update();
                this.updateActiveButton('show-all');
            },

            show(category) {
                AppState.currentCategory = category;
                AppState.pagination.currentPage = 1; // Reset pagination
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

                let allChannels = [];

                // First, collect all channels that should be visible based on category and search
                DOMCache.sections.forEach(section => {
                    const category = section.dataset.category;
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === category;

                    if (shouldShowCategory) {
                        const channels = this.getChannelsInSection(section);

                        // Add all matching channels to the global list
                        allChannels = allChannels.concat(channels);
                    }
                });

                // Apply global pagination to all collected channels
                PaginationManager.updatePagination(allChannels);

                // Show/hide no results message
                const hasVisibleResults = this.checkForVisibleResults();
                if (hasVisibleResults) {
                    Utils.hideElement(DOMCache.noResults);
                } else {
                    Utils.showElement(DOMCache.noResults);
                }
            },

            checkForVisibleResults() {
                // Check if any sections have visible channels after pagination
                let hasResults = false;
                DOMCache.sections.forEach(section => {
                    const visibleChannels = section.querySelectorAll('.c:not([style*="display: none"])');
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === section.dataset.category;

                    if (shouldShowCategory && visibleChannels.length > 0) {
                        hasResults = true;
                    }
                });                return hasResults;
            },

            getChannelsInSection(section) {
                const channels = section.querySelectorAll('.c');
                return Array.from(channels); // Return all channels, let pagination handle filtering
            },            filterChannelsInSection(section) {
                const channels = section.querySelectorAll('.c');
                let hasVisibleChannels = false;

                channels.forEach(channel => {
                    const channelName = channel.dataset.n;
                    const matchesSearch = AppState.searchTerm === '' ||
                                        (channelName && channelName.toLowerCase().includes(AppState.searchTerm));

                    if (matchesSearch) {
                        hasVisibleChannels = true;
                    }
                });return hasVisibleChannels;
            },            // Optimized update method for large datasets
            updateOptimized() {
                console.time('ViewManager.updateOptimized');

                if (!DOMCache.sections || !DOMCache.noResults) return;

                // Use cached channel data for better performance
                const filteredChannelData = this.getFilteredChannelData();

                // Convert filtered channel data back to DOM elements for pagination
                const filteredChannelElements = filteredChannelData.map(ch => ch.element);

                // Apply global pagination using regular method
                PaginationManager.updatePagination(filteredChannelElements);

                // Show/hide no results message
                if (filteredChannelData.length > 0) {
                    Utils.hideElement(DOMCache.noResults);
                } else {
                    Utils.showElement(DOMCache.noResults);
                }

                console.timeEnd('ViewManager.updateOptimized');
            },

            getFilteredChannelData() {
                if (!DOMCache.allChannelData || DOMCache.allChannelData.length === 0) {
                    // Fallback to regular method if cache is empty
                    DOMCache.cacheChannelData();
                }

                return DOMCache.allChannelData.filter(channelData => {
                    const categoryMatch = AppState.currentCategory === 'all' ||
                                        AppState.currentCategory === channelData.category;
                    const searchMatch = AppState.searchTerm === '' ||
                                      channelData.name.includes(AppState.searchTerm);

                    return categoryMatch && searchMatch;
                });
            },            batchUpdateSections(filteredChannelData) {
                const startIndex = (AppState.pagination.currentPage - 1) * AppState.pagination.pageSize;
                const endIndex = startIndex + AppState.pagination.pageSize;
                const visibleChannelIds = new Set(
                    filteredChannelData
                        .slice(startIndex, endIndex)
                        .map(ch => ch.id)
                );

                // Batch DOM updates to minimize reflows
                const updates = [];

                DOMCache.sections.forEach(section => {
                    const category = section.dataset.category;
                    const shouldShowCategory = AppState.currentCategory === 'all' ||
                                             AppState.currentCategory === category;

                    if (shouldShowCategory) {
                        const channels = section.querySelectorAll('.c');
                        let hasVisibleChannels = false;

                        channels.forEach(channel => {
                            const channelId = channel.dataset.i; // Use 'i' not 'id'
                            if (visibleChannelIds.has(channelId)) {
                                updates.push(() => Utils.showElement(channel));
                                hasVisibleChannels = true;
                            } else {
                                updates.push(() => Utils.hideElement(channel));
                            }
                        });

                        // Update section visibility and count - always hide sections without visible channels
                        if (hasVisibleChannels) {
                            updates.push(() => Utils.showElement(section));

                            const countSpan = section.querySelector('.channel-count');
                            if (countSpan) {
                                const totalInCategory = channels.length;
                                updates.push(() => countSpan.textContent = totalInCategory);
                            }
                        } else {
                            // Hide section if no channels are visible on current page
                            updates.push(() => Utils.hideElement(section));
                        }
                    } else {
                        updates.push(() => Utils.hideElement(section));
                    }
                });

                // Execute all DOM updates in a single batch
                updates.forEach(update => update());

                // Update pagination controls visibility
                const paginationContainer = document.querySelector('.global-pagination');
                if (paginationContainer) {
                    if (AppState.pagination.totalPages > 1) {
                        Utils.showElement(paginationContainer, 'flex');
                    } else {
                        Utils.hideElement(paginationContainer);
                    }
                }
            }
        };        // Pagination management
        const PaginationManager = {
            updatePagination(allChannels) {
                const filteredChannels = allChannels.filter(channel => {
                    const channelName = channel.dataset.n;
                    return AppState.searchTerm === '' ||
                           (channelName && channelName.toLowerCase().includes(AppState.searchTerm));
                });

                AppState.pagination.totalChannels = filteredChannels.length;
                AppState.pagination.totalPages = Math.ceil(filteredChannels.length / AppState.pagination.pageSize);

                // Reset to page 1 if current page is beyond total pages
                if (AppState.pagination.currentPage > AppState.pagination.totalPages) {
                    AppState.pagination.currentPage = 1;
                }

                // Ensure current page is at least 1
                if (AppState.pagination.currentPage < 1) {
                    AppState.pagination.currentPage = 1;
                }

                this.showPage(filteredChannels);
                this.updatePaginationUI();
                this.updateChannelCounts();
            },

            showPage(filteredChannels) {
                const startIndex = (AppState.pagination.currentPage - 1) * AppState.pagination.pageSize;
                const endIndex = startIndex + AppState.pagination.pageSize;

                // Hide all channels first
                const allChannels = document.querySelectorAll('.c');
                allChannels.forEach(channel => Utils.hideElement(channel));

                // Show only channels for current page
                const channelsToShow = filteredChannels.slice(startIndex, endIndex);
                channelsToShow.forEach(channel => Utils.showElement(channel));

                AppState.pagination.visibleChannels = channelsToShow.length;

                // Update sections visibility based on whether they have visible channels
                this.updateSectionVisibility();
            },            updateSectionVisibility() {
                DOMCache.sections.forEach(section => {
                    const visibleChannels = section.querySelectorAll('.c:not([style*="display: none"])');
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === section.dataset.category;

                    if (shouldShowCategory && visibleChannels.length > 0) {
                        Utils.showElement(section);
                        // Update channel count in title
                        const countSpan = section.querySelector('.channel-count');
                        if (countSpan) {
                            const totalInCategory = section.querySelectorAll('.c').length;
                            countSpan.textContent = totalInCategory;
                        }                        // Show pagination controls for global pagination
                        const paginationContainer = document.querySelector('.global-pagination');
                        if (paginationContainer) {
                            if (AppState.pagination.totalPages > 1) {
                                Utils.showElement(paginationContainer, 'flex');
                            } else {
                                Utils.hideElement(paginationContainer);
                            }
                        }
                    } else {
                        // Hide section if no channels are visible (regardless of search state)
                        Utils.hideElement(section);
                    }
                });
            },updatePaginationUI() {
                const paginationContainer = document.querySelector('.global-pagination');
                if (!paginationContainer) return;

                // Update page info
                const currentPageSpan = paginationContainer.querySelector('.current-page');
                const totalPagesSpan = paginationContainer.querySelector('.total-pages');
                const visibleChannelsSpan = paginationContainer.querySelector('.visible-channels');
                const totalChannelsSpan = paginationContainer.querySelector('.total-channels');

                if (currentPageSpan) currentPageSpan.textContent = AppState.pagination.currentPage;
                if (totalPagesSpan) totalPagesSpan.textContent = AppState.pagination.totalPages;
                if (visibleChannelsSpan) visibleChannelsSpan.textContent = AppState.pagination.visibleChannels;
                if (totalChannelsSpan) totalChannelsSpan.textContent = AppState.pagination.totalChannels;

                // Update pagination controls
                this.updatePaginationControls(paginationContainer);
            },

            updatePaginationControls(container) {
                const controls = container.querySelector('.pagination-controls');
                if (!controls) return;

                const buttons = controls.querySelectorAll('.pagination-btn');
                const pageNumbersDiv = controls.querySelector('.page-numbers');

                // Update navigation buttons
                buttons[0].disabled = AppState.pagination.currentPage === 1; // First
                buttons[1].disabled = AppState.pagination.currentPage === 1; // Previous
                buttons[buttons.length - 2].disabled = AppState.pagination.currentPage === AppState.pagination.totalPages; // Next
                buttons[buttons.length - 1].disabled = AppState.pagination.currentPage === AppState.pagination.totalPages; // Last

                // Generate page numbers
                this.generatePageNumbers(pageNumbersDiv);
            },

            generatePageNumbers(container) {
                if (!container) return;

                container.innerHTML = '';

                if (AppState.pagination.totalPages <= 1) return;

                const currentPage = AppState.pagination.currentPage;
                const totalPages = AppState.pagination.totalPages;

                // Calculate which pages to show
                let startPage = Math.max(1, currentPage - 2);
                let endPage = Math.min(totalPages, currentPage + 2);

                // Adjust range to always show 5 pages when possible
                if (endPage - startPage < 4) {
                    if (startPage === 1) {
                        endPage = Math.min(totalPages, startPage + 4);
                    } else if (endPage === totalPages) {
                        startPage = Math.max(1, endPage - 4);
                    }
                }

                // Add first page and ellipsis if needed
                if (startPage > 1) {
                    this.addPageButton(container, 1);
                    if (startPage > 2) {
                        container.appendChild(this.createEllipsis());
                    }
                }

                // Add page numbers
                for (let i = startPage; i <= endPage; i++) {
                    this.addPageButton(container, i, i === currentPage);
                }

                // Add last page and ellipsis if needed
                if (endPage < totalPages) {
                    if (endPage < totalPages - 1) {
                        container.appendChild(this.createEllipsis());
                    }
                    this.addPageButton(container, totalPages);
                }
            },

            addPageButton(container, pageNumber, isActive = false) {
                const button = document.createElement('button');
                button.className = 'pagination-btn' + (isActive ? ' active' : '');
                button.textContent = pageNumber;
                button.onclick = () => this.goToPage(pageNumber);
                container.appendChild(button);
            },

            createEllipsis() {
                const span = document.createElement('span');
                span.className = 'pagination-ellipsis';
                span.textContent = '...';
                span.style.cssText = 'padding: 0 8px; color: var(--text-muted);';
                return span;
            },

            goToPage(pageNumber) {
                AppState.pagination.currentPage = pageNumber;
                ViewManager.update();
                this.scrollToTop();
            },

            goToFirstPage() {
                this.goToPage(1);
            },

            goToPrevPage() {
                if (AppState.pagination.currentPage > 1) {
                    this.goToPage(AppState.pagination.currentPage - 1);
                }
            },

            goToNextPage() {
                if (AppState.pagination.currentPage < AppState.pagination.totalPages) {
                    this.goToPage(AppState.pagination.currentPage + 1);
                }
            },

            goToLastPage() {
                this.goToPage(AppState.pagination.totalPages);
            },

            updatePageSize(newSize) {
                AppState.pagination.pageSize = parseInt(newSize);
                AppState.pagination.currentPage = 1; // Reset to first page
                ViewManager.update();
            },

            updateChannelCounts() {
                // Update the search results info if needed
                const searchInfo = document.querySelector('.search-results-info');
                if (searchInfo) {
                    searchInfo.textContent = AppState.pagination.totalChannels + ' channels found';
                }
            },            scrollToTop() {
                Utils.scrollToElement(DOMCache.mainContent);
            }
        };// Dynamic content generator for compact cards
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

                    card.innerHTML = logoHtml + '<h3 class="channel-name">' + this.escapeHtml(displayName) + '</h3><span class="channel-id" onclick="copyChannelId(\'' + this.escapeHtml(id) + '\')" title="Click to copy Discord command">/tv channel:' + this.escapeHtml(id) + ' <span class="copy-icon">📋</span><div class="copy-feedback">Command copied!</div></span>';
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
        };        // Global functions (needed for inline onclick handlers)
        window.clearSearch = () => SearchManager.clear();
        window.copyChannelId = (channelId) => ClipboardManager.copy(channelId, window.event);
        window.copyPlaylistId = (playlistName) => ClipboardManager.copyPlaylist(playlistName, window.event);
        window.toggleCategoriesView = () => CategoryManager.toggleView();
        window.toggleCategory = (category) => CategoryManager.toggle(category);
        window.goToFirstPage = () => PaginationManager.goToFirstPage();
        window.goToPrevPage = () => PaginationManager.goToPrevPage();
        window.goToNextPage = () => PaginationManager.goToNextPage();
        window.goToLastPage = () => PaginationManager.goToLastPage();
        window.updatePageSize = (newSize) => PaginationManager.updatePageSize(newSize);

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

// Methods for embedded JSON approach

func (c *CatalogGenerator) buildHeaderForEmbeddedJSON() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IPTV Catalog</title>
    <meta name="description" content="IPTV Channel Catalog with Enhanced Performance">
`
}

func (c *CatalogGenerator) buildNavigationForEmbeddedJSON() string {
	return `    <header>
        <div class="container">
            <h1>IPTV Channel Catalog</h1>
            <div class="subtitle" id="playlistSubtitle"></div>
            <div class="playlist-command">
                <span class="playlist-id" id="playlistCommand" title="Click to copy Discord command">
                    <span class="copy-icon">📋</span>
                    <div class="copy-feedback">Command copied!</div>
                </span>
            </div>
            <div class="stats" id="catalogStats"></div>
        </div>
    </header>

    <div class="container">
        <div class="search-container">
            <div class="search-wrapper">
                <input type="text" id="searchBox" class="search-box" placeholder="Search channels...">
                <button class="clear-search" id="clearSearch" onclick="clearSearch()" title="Clear search">✕</button>
            </div>
        </div>

        <nav class="categories-nav collapsed" id="categoriesNav">
            <div class="categories-grid collapsed" id="categoriesGrid">
                <button class="category-btn show-all-btn active" onclick="toggleCategoriesView()" id="showAllBtn">
                    Show All Categories <span class="expand-icon">▼</span>
                </button>
                <!-- Categories will be populated by JavaScript -->
            </div>
        </nav>
`
}

func (c *CatalogGenerator) buildMainContentForEmbeddedJSON() string {
	return `<main id="mainContent">
    <!-- Global pagination controls -->
    <div class="pagination-container global-pagination">
        <div class="page-size-selector">
            <label>Per page:</label>
            <select class="page-size-select" onchange="updatePageSize(this.value)">
                <option value="50">50</option>
                <option value="100">100</option>
                <option value="200" selected>200</option>
                <option value="500">500</option>
            </select>
        </div>
        <div class="pagination-info">
            <span class="page-info">Page <span class="current-page">1</span> of <span class="total-pages">1</span></span>
            <span class="results-info">(<span class="visible-channels">0</span> of <span class="total-channels">0</span> channels)</span>
        </div>
        <div class="pagination-controls">
            <button class="pagination-btn" onclick="goToFirstPage()" title="First page">⏮</button>
            <button class="pagination-btn" onclick="goToPrevPage()" title="Previous page">◀</button>
            <div class="page-numbers"></div>
            <button class="pagination-btn" onclick="goToNextPage()" title="Next page">▶</button>
            <button class="pagination-btn" onclick="goToLastPage()" title="Last page">⏭</button>
        </div>
    </div>

    <!-- Categories and channels will be populated by JavaScript -->
    <div id="categoriesContainer"></div>

    <div id="noResults" class="no-results" style="display:none;">
        <h3>No channels found</h3>
        <p>Try adjusting your search terms or selecting a different category.</p>
    </div>
</main>

<footer class="footer">
    <div class="container">
        Generated by Discord IPTV Player • <span id="generatedDate"></span>
    </div>
</footer>
</div>
`
}

func (c *CatalogGenerator) buildEmbeddedJSONScript(jsonData string) string {
	return fmt.Sprintf(`<script type="application/json" id="embedded-channel-data">%s</script>`, jsonData)
}

func (c *CatalogGenerator) buildJavaScriptForEmbeddedJSON() string {
	return `    <script>
        // Load data from embedded JSON with compact structure
        let catalogData;
        try {
            const rawData = JSON.parse(document.getElementById('embedded-channel-data').textContent);
            // Transform compact JSON structure back to readable format
            catalogData = {
                playlistName: rawData.p || 'Unknown',
                date: rawData.d || new Date().toDateString(),
                categories: rawData.c || [],
                categoryChannels: rawData.cc || {},
                totalChannels: rawData.t || 0
            };
        } catch (e) {
            console.error('Failed to parse embedded JSON data:', e);
            catalogData = {
                playlistName: 'Unknown',
                date: new Date().toDateString(),
                categories: [],
                categoryChannels: {},
                totalChannels: 0
            };
        }

        // Application state
        const AppState = {
            currentCategory: 'all',
            searchTerm: '',
            categoriesExpanded: false,
            searchTimeout: null,
            pagination: {
                currentPage: 1,
                pageSize: 200,
                totalPages: 1,
                totalChannels: 0,
                visibleChannels: 0
            }
        };

        // Enhanced DOM cache
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
            categoriesContainer: null,

            // Performance optimization: Cache channel data
            allChannelData: [],
            visibleChannelElements: new Map(),
            sectionChannelMap: new Map(),

            init() {
                this.searchBox = document.getElementById('searchBox');
                this.clearBtn = document.getElementById('clearSearch');
                this.categoriesGrid = document.getElementById('categoriesGrid');
                this.categoriesNav = document.getElementById('categoriesNav');
                this.showAllBtn = document.getElementById('showAllBtn');
                this.noResults = document.getElementById('noResults');
                this.mainContent = document.getElementById('mainContent');
                this.categoriesContainer = document.getElementById('categoriesContainer');

                // Initialize the page with catalog data
                this.initializePage();
                this.buildCategoriesNavigation();
                this.buildChannelSections();
                this.cacheChannelData();

                // Cache NodeLists after building content
                this.sections = document.querySelectorAll('.category-section');
                this.categoryBtns = document.querySelectorAll('.category-btn');
            },

            initializePage() {
                // Set playlist information
                const playlistSubtitle = document.getElementById('playlistSubtitle');
                const playlistCommand = document.getElementById('playlistCommand');
                const catalogStats = document.getElementById('catalogStats');
                const generatedDate = document.getElementById('generatedDate');

                if (playlistSubtitle) playlistSubtitle.textContent = catalogData.playlistName;
                if (catalogStats) catalogStats.textContent = catalogData.categories.length + ' categories • ' + catalogData.totalChannels + ' channels • Generated on ' + catalogData.date;
                if (generatedDate) generatedDate.textContent = catalogData.date;

                if (playlistCommand) {
                    playlistCommand.innerHTML = '/playlist name:' + this.escapeHtml(catalogData.playlistName) + ' <span class="copy-icon">📋</span><div class="copy-feedback">Command copied!</div>';
                    playlistCommand.onclick = (event) => ClipboardManager.copyPlaylist(catalogData.playlistName, event);
                }

                // Update show all button
                if (this.showAllBtn) {
                    this.showAllBtn.innerHTML = 'Show All Categories (' + catalogData.categories.length + ') <span class="expand-icon">▼</span>';
                }
            },

            buildCategoriesNavigation() {
                if (!this.categoriesGrid) return;

                catalogData.categories.forEach(category => {
                    const channelCount = catalogData.categoryChannels[category] ? catalogData.categoryChannels[category].length : 0;
                    const button = document.createElement('button');
                    button.className = 'category-btn';
                    button.textContent = category + ' (' + channelCount + ')';
                    button.onclick = () => CategoryManager.toggle(category);
                    this.categoriesGrid.appendChild(button);
                });
            },

            buildChannelSections() {
                if (!this.categoriesContainer) return;

                catalogData.categories.forEach(category => {
                    const channels = catalogData.categoryChannels[category] || [];
                    if (channels.length === 0) return;

                    // Sort channels by name (using compact structure: n = name)
                    channels.sort((a, b) => a.n.localeCompare(b.n));

                    const section = document.createElement('section');
                    section.className = 'category-section';
                    section.setAttribute('data-category', category);

                    const title = document.createElement('h2');
                    title.className = 'category-title';
                    title.innerHTML = this.escapeHtml(category) + ' <span style="font-weight:400;color:#666;">(<span class="channel-count">' + channels.length + '</span> channels)</span>';

                    const channelsGrid = document.createElement('div');
                    channelsGrid.className = 'channels-grid';

                    channels.forEach(channel => {
                        const channelCard = this.createChannelCard(channel);
                        channelsGrid.appendChild(channelCard);
                    });

                    section.appendChild(title);
                    section.appendChild(channelsGrid);
                    this.categoriesContainer.appendChild(section);
                });
            },

            createChannelCard(channel) {
                const card = document.createElement('div');
                card.className = 'c';
                // Use compact JSON structure (i = id, n = name, l = logo)
                card.setAttribute('data-n', channel.n.toLowerCase());
                card.setAttribute('data-i', channel.i);
                if (channel.l) {
                    card.setAttribute('data-logo', channel.l);
                }
                return card;
            },

            cacheChannelData() {
                console.time('Channel data caching');
                this.allChannelData = [];
                this.sectionChannelMap.clear();

                this.sections = document.querySelectorAll('.category-section');
                this.sections.forEach(section => {
                    const category = section.dataset.category;
                    const channels = Array.from(section.querySelectorAll('.c'));

                    // Cache channels by category
                    this.sectionChannelMap.set(category, channels);

                    // Cache channel data for faster searching
                    channels.forEach(channel => {
                        this.allChannelData.push({
                            element: channel,
                            name: (channel.dataset.n || '').toLowerCase(),
                            category: category,
                            id: channel.dataset.i,
                            logo: channel.dataset.logo
                        });
                    });
                });

                console.timeEnd('Channel data caching');
                console.log('Cached ' + this.allChannelData.length + ' channels across ' + this.sections.length + ' categories');
            },

            refresh() {
                const currentSectionCount = document.querySelectorAll('.category-section').length;
                if (currentSectionCount !== this.sections.length) {
                    this.sections = document.querySelectorAll('.category-section');
                    this.categoryBtns = document.querySelectorAll('.category-btn');
                    this.cacheChannelData();
                }
            },

            escapeHtml(text) {
                const div = document.createElement('div');
                div.textContent = text;
                return div.innerHTML;
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
                AppState.pagination.currentPage = 1;
                SearchManager.toggleClearButton();

                if (DOMCache.allChannelData.length > 1000) {
                    ViewManager.updateOptimized();
                } else {
                    ViewManager.update();
                }
            }, 150)
        };

        // Clipboard functionality
        const ClipboardManager = {
            async copy(channelId, event) {
                if (!event) return;

                const clickedElement = event.target.closest('.channel-id');
                const feedback = clickedElement?.querySelector('.copy-feedback');

                if (!clickedElement || !feedback) return;

                const discordCommand = '/tv channel:' + channelId;

                try {
                    if (navigator.clipboard && window.isSecureContext) {
                        await navigator.clipboard.writeText(discordCommand);
                    } else {
                        this.fallbackCopy(discordCommand);
                    }
                    this.showFeedback(clickedElement, feedback);
                } catch (err) {
                    console.warn('Clipboard copy failed, using fallback:', err);
                    this.fallbackCopy(discordCommand);
                    this.showFeedback(clickedElement, feedback);
                }
            },

            async copyPlaylist(playlistName, event) {
                if (!event) return;

                const clickedElement = event.target.closest('.playlist-id');
                const feedback = clickedElement?.querySelector('.copy-feedback');

                if (!clickedElement || !feedback) return;

                const discordCommand = '/playlist name:' + playlistName;

                try {
                    if (navigator.clipboard && window.isSecureContext) {
                        await navigator.clipboard.writeText(discordCommand);
                    } else {
                        this.fallbackCopy(discordCommand);
                    }
                    this.showFeedback(clickedElement, feedback);
                } catch (err) {
                    console.warn('Clipboard copy failed, using fallback:', err);
                    this.fallbackCopy(discordCommand);
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

                element.classList.add('copied');
                feedback.classList.add('show');

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
            },

            updateButtonText(text, arrow) {
                if (!DOMCache.showAllBtn) return;

                DOMCache.showAllBtn.innerHTML = text + ' (' + catalogData.categories.length + ') <span class="expand-icon">' + arrow + '</span>';
            },

            showAll() {
                AppState.currentCategory = 'all';
                AppState.pagination.currentPage = 1;
                ViewManager.update();
                this.updateActiveButton('show-all');
            },

            show(category) {
                AppState.currentCategory = category;
                AppState.pagination.currentPage = 1;
                ViewManager.update();
                this.updateActiveButton(category);

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

                DOMCache.categoryBtns.forEach(btn => btn.classList.remove('active'));

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
        };

        // View management
        const ViewManager = {
            update() {
                if (!DOMCache.sections || !DOMCache.noResults) return;

                let allChannels = [];

                DOMCache.sections.forEach(section => {
                    const category = section.dataset.category;
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === category;

                    if (shouldShowCategory) {
                        const channels = this.getChannelsInSection(section);
                        allChannels = allChannels.concat(channels);
                    }
                });

                PaginationManager.updatePagination(allChannels);

                const hasVisibleResults = this.checkForVisibleResults();
                if (hasVisibleResults) {
                    Utils.hideElement(DOMCache.noResults);
                } else {
                    Utils.showElement(DOMCache.noResults);
                }
            },

            checkForVisibleResults() {
                let hasResults = false;
                DOMCache.sections.forEach(section => {
                    const visibleChannels = section.querySelectorAll('.c:not([style*="display: none"])');
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === section.dataset.category;

                    if (shouldShowCategory && visibleChannels.length > 0) {
                        hasResults = true;
                    }
                });

                return hasResults;
            },

            getChannelsInSection(section) {
                const channels = section.querySelectorAll('.c');
                return Array.from(channels);
            },

            updateOptimized() {
                console.time('ViewManager.updateOptimized');

                if (!DOMCache.sections || !DOMCache.noResults) return;

                const filteredChannelData = this.getFilteredChannelData();
                const filteredChannelElements = filteredChannelData.map(ch => ch.element);

                PaginationManager.updatePagination(filteredChannelElements);

                if (filteredChannelData.length > 0) {
                    Utils.hideElement(DOMCache.noResults);
                } else {
                    Utils.showElement(DOMCache.noResults);
                }

                console.timeEnd('ViewManager.updateOptimized');
            },

            getFilteredChannelData() {
                if (!DOMCache.allChannelData || DOMCache.allChannelData.length === 0) {
                    DOMCache.cacheChannelData();
                }

                return DOMCache.allChannelData.filter(channelData => {
                    const categoryMatch = AppState.currentCategory === 'all' ||
                                        AppState.currentCategory === channelData.category;
                    const searchMatch = AppState.searchTerm === '' ||
                                      channelData.name.includes(AppState.searchTerm);

                    return categoryMatch && searchMatch;
                });
            }
        };

        // Pagination management
        const PaginationManager = {
            updatePagination(allChannels) {
                const filteredChannels = allChannels.filter(channel => {
                    const channelName = channel.dataset.n;
                    return AppState.searchTerm === '' ||
                           (channelName && channelName.toLowerCase().includes(AppState.searchTerm));
                });

                AppState.pagination.totalChannels = filteredChannels.length;
                AppState.pagination.totalPages = Math.ceil(filteredChannels.length / AppState.pagination.pageSize);

                if (AppState.pagination.currentPage > AppState.pagination.totalPages) {
                    AppState.pagination.currentPage = 1;
                }

                if (AppState.pagination.currentPage < 1) {
                    AppState.pagination.currentPage = 1;
                }

                this.showPage(filteredChannels);
                this.updatePaginationUI();
                this.updateChannelCounts();
            },

            showPage(filteredChannels) {
                const startIndex = (AppState.pagination.currentPage - 1) * AppState.pagination.pageSize;
                const endIndex = startIndex + AppState.pagination.pageSize;

                const allChannels = document.querySelectorAll('.c');
                allChannels.forEach(channel => Utils.hideElement(channel));

                const channelsToShow = filteredChannels.slice(startIndex, endIndex);
                channelsToShow.forEach(channel => Utils.showElement(channel));

                AppState.pagination.visibleChannels = channelsToShow.length;

                this.updateSectionVisibility();
            },

            updateSectionVisibility() {
                DOMCache.sections.forEach(section => {
                    const visibleChannels = section.querySelectorAll('.c:not([style*="display: none"])');
                    const shouldShowCategory = AppState.currentCategory === 'all' || AppState.currentCategory === section.dataset.category;

                    if (shouldShowCategory && visibleChannels.length > 0) {
                        Utils.showElement(section);
                        const countSpan = section.querySelector('.channel-count');
                        if (countSpan) {
                            const totalInCategory = section.querySelectorAll('.c').length;
                            countSpan.textContent = totalInCategory;
                        }

                        const paginationContainer = document.querySelector('.global-pagination');
                        if (paginationContainer) {
                            if (AppState.pagination.totalPages > 1) {
                                Utils.showElement(paginationContainer, 'flex');
                            } else {
                                Utils.hideElement(paginationContainer);
                            }
                        }
                    } else {
                        Utils.hideElement(section);
                    }
                });
            },

            updatePaginationUI() {
                const paginationContainer = document.querySelector('.global-pagination');
                if (!paginationContainer) return;

                const currentPageSpan = paginationContainer.querySelector('.current-page');
                const totalPagesSpan = paginationContainer.querySelector('.total-pages');
                const visibleChannelsSpan = paginationContainer.querySelector('.visible-channels');
                const totalChannelsSpan = paginationContainer.querySelector('.total-channels');

                if (currentPageSpan) currentPageSpan.textContent = AppState.pagination.currentPage;
                if (totalPagesSpan) totalPagesSpan.textContent = AppState.pagination.totalPages;
                if (visibleChannelsSpan) visibleChannelsSpan.textContent = AppState.pagination.visibleChannels;
                if (totalChannelsSpan) totalChannelsSpan.textContent = AppState.pagination.totalChannels;

                this.updatePaginationControls(paginationContainer);
            },

            updatePaginationControls(container) {
                const controls = container.querySelector('.pagination-controls');
                if (!controls) return;

                const buttons = controls.querySelectorAll('.pagination-btn');
                const pageNumbersDiv = controls.querySelector('.page-numbers');

                buttons[0].disabled = AppState.pagination.currentPage === 1;
                buttons[1].disabled = AppState.pagination.currentPage === 1;
                buttons[buttons.length - 2].disabled = AppState.pagination.currentPage === AppState.pagination.totalPages;
                buttons[buttons.length - 1].disabled = AppState.pagination.currentPage === AppState.pagination.totalPages;

                this.generatePageNumbers(pageNumbersDiv);
            },

            generatePageNumbers(container) {
                if (!container) return;

                container.innerHTML = '';

                if (AppState.pagination.totalPages <= 1) return;

                const currentPage = AppState.pagination.currentPage;
                const totalPages = AppState.pagination.totalPages;

                let startPage = Math.max(1, currentPage - 2);
                let endPage = Math.min(totalPages, currentPage + 2);

                if (endPage - startPage < 4) {
                    if (startPage === 1) {
                        endPage = Math.min(totalPages, startPage + 4);
                    } else if (endPage === totalPages) {
                        startPage = Math.max(1, endPage - 4);
                    }
                }

                if (startPage > 1) {
                    this.addPageButton(container, 1);
                    if (startPage > 2) {
                        container.appendChild(this.createEllipsis());
                    }
                }

                for (let i = startPage; i <= endPage; i++) {
                    this.addPageButton(container, i, i === currentPage);
                }

                if (endPage < totalPages) {
                    if (endPage < totalPages - 1) {
                        container.appendChild(this.createEllipsis());
                    }
                    this.addPageButton(container, totalPages);
                }
            },

            addPageButton(container, pageNumber, isActive = false) {
                const button = document.createElement('button');
                button.className = 'pagination-btn' + (isActive ? ' active' : '');
                button.textContent = pageNumber;
                button.onclick = () => this.goToPage(pageNumber);
                container.appendChild(button);
            },

            createEllipsis() {
                const span = document.createElement('span');
                span.className = 'pagination-ellipsis';
                span.textContent = '...';
                span.style.cssText = 'padding: 0 8px; color: var(--text-muted);';
                return span;
            },

            goToPage(pageNumber) {
                AppState.pagination.currentPage = pageNumber;
                ViewManager.update();
                this.scrollToTop();
            },

            goToFirstPage() {
                this.goToPage(1);
            },

            goToPrevPage() {
                if (AppState.pagination.currentPage > 1) {
                    this.goToPage(AppState.pagination.currentPage - 1);
                }
            },

            goToNextPage() {
                if (AppState.pagination.currentPage < AppState.pagination.totalPages) {
                    this.goToPage(AppState.pagination.currentPage + 1);
                }
            },

            goToLastPage() {
                this.goToPage(AppState.pagination.totalPages);
            },

            updatePageSize(newSize) {
                AppState.pagination.pageSize = parseInt(newSize);
                AppState.pagination.currentPage = 1;
                ViewManager.update();
            },

            updateChannelCounts() {
                const searchInfo = document.querySelector('.search-results-info');
                if (searchInfo) {
                    searchInfo.textContent = AppState.pagination.totalChannels + ' channels found';
                }
            },

            scrollToTop() {
                Utils.scrollToElement(DOMCache.mainContent);
            }
        };

        // Dynamic content generator for compact cards
        const ContentGenerator = {
            generateChannelCards() {
                document.querySelectorAll('.c').forEach(card => {
                    if (card.innerHTML) return;

                    const name = card.dataset.n;
                    const id = card.dataset.i;
                    const logo = card.dataset.logo;

                    const displayName = this.capitalize(name);
                    const firstLetter = displayName.charAt(0).toUpperCase();

                    const logoHtml = logo ?
                        '<img src="' + this.escapeHtml(logo) + '" alt="' + this.escapeHtml(displayName) + ' Logo" class="channel-logo" onerror="this.style.display=\'none\';this.nextElementSibling.style.display=\'flex\';"><div class="no-logo" style="display:none;">' + firstLetter + '</div>' :
                        '<div class="no-logo">' + firstLetter + '</div>';

                    card.innerHTML = logoHtml + '<h3 class="channel-name">' + this.escapeHtml(displayName) + '</h3><span class="channel-id" onclick="copyChannelId(\'' + this.escapeHtml(id) + '\')" title="Click to copy Discord command">/tv channel:' + this.escapeHtml(id) + ' <span class="copy-icon">📋</span><div class="copy-feedback">Command copied!</div></span>';
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
                if (event.key === 'Escape' && AppState.searchTerm) {
                    event.preventDefault();
                    SearchManager.clear();
                }

                if ((event.ctrlKey || event.metaKey) && event.key === 'f') {
                    event.preventDefault();
                    DOMCache.searchBox?.focus();
                }
            }
        };

        // Global functions (needed for inline onclick handlers)
        window.clearSearch = () => SearchManager.clear();
        window.copyChannelId = (channelId) => ClipboardManager.copy(channelId, window.event);
        window.copyPlaylistId = (playlistName) => ClipboardManager.copyPlaylist(playlistName, window.event);
        window.toggleCategoriesView = () => CategoryManager.toggleView();
        window.toggleCategory = (category) => CategoryManager.toggle(category);
        window.goToFirstPage = () => PaginationManager.goToFirstPage();
        window.goToPrevPage = () => PaginationManager.goToPrevPage();
        window.goToNextPage = () => PaginationManager.goToNextPage();
        window.goToLastPage = () => PaginationManager.goToLastPage();
        window.updatePageSize = (newSize) => PaginationManager.updatePageSize(newSize);

        // Event listeners
        function initializeEventListeners() {
            DOMCache.searchBox?.addEventListener('input', (e) => {
                SearchManager.handleInput(e.target.value);
            });

            DOMCache.categoryBtns?.forEach(btn => {
                btn.addEventListener('click', () => {
                    setTimeout(() => {
                        Utils.scrollToElement(DOMCache.mainContent);
                    }, 100);
                });
            });

            KeyboardManager.init();
        }

        // Application initialization
        function initializeApp() {
            DOMCache.init();
            ContentGenerator.generateChannelCards();
            SearchManager.toggleClearButton();
            ViewManager.update();
            initializeEventListeners();
        }

        // Initialize when DOM is ready
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', initializeApp);
        } else {
            initializeApp();
        }
    </script>`
}
