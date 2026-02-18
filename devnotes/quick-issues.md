# Quick Issues

## Project gsi

- Upgrade bmad installation to support 6.0 release. `bunx bmad-method install --directory ./ --modules bmm --tools claude-code --yes` is the command to run. This will install the latest version of bmad and the claude-code tool, which is needed for the gsi project.

## Project berkshire

1. Dashboard ux/ui redesign: The current dashboard is functional but could be improved in terms of user experience and interface design. Note: to help with ui/ux redesign and inspection, you can run `uvx rodney --help` to see the available commands and options for interacting with the web via Chrome automation. You can use rodney to perform tasks such as validating web pages, taking screenshots, and more, all from the command line.
   1.1 it is a long page with just a bunch of signals, often taken up by a single company.
   1.2 It is hard to find which companies we're tracking (watchlist), a concise list would be best at the top with some indication of which companies have signals that have been updated recently and how they're performing.
   1.3 I like the visualizations - maybe they can be laid out to be more visible vs. beneath the fold.
2. Main navigation - some of the navigation items might not make sense. I.e. Sources list and configurator could likely go under settings page, Careers page is just a source, maybe there's a dropdown nav item "Sources" that lists the detail pages for each of the sources. Maybe there are some other more prominent items that should go in the Nav. Should consider and possibly rework the information architecture flow and how users drill down into different companies and their signals.
3. There should be a distinction between the date a source or signal was imported, and the date it was published. Many of the job postings are imported after they were published, so the date they were published is more relevant to understanding the timeline of signals for a company. This is especially important for the careers page signals, which are often imported in bulk and can be months old. Likewise, SEC filings are often imported in bulk and can be months old, so the date they were published is more relevant than the date they were imported. Consider identifying all data sources or signals that should track the published date vs. the imported date and make sure that distinction is clear in the UI and in the data model.
