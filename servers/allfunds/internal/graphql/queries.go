package graphql

// GraphQL queries for Allfunds Connect API
// Ported from Python server at ../../allfunds-mcp/server.py

// ScreenProductsQuery searches for funds/ETPs by criteria
const ScreenProductsQuery = `
query ScreenProducts($screeningCriteria: ScreeningCriteria!, $pagination: Pagination!, $order: ProductOrder!, $cache: Boolean!) {
  product_screener {
    screen_products(criteria: $screeningCriteria pagination: $pagination order: $order cache: $cache) {
      results {
        id
        isin
        isin_bc
        allfunds_id
        product_type
        name
        dealable
        currency
        dealable_for_entity
        service_type
        performance_by_periods
        ratios: calculated_ratios
        data(with_companies: false)
        due_diligence_label
        latest_price { date value }
      }
      pagination_result { page_count total_count }
    }
  }
}
`

// GetProductQuery retrieves complete product details by internal ID
const GetProductQuery = `
query GetProduct($id: ID!) {
  product(id: $id) {
    id
    isin
    isin_bc
    allfunds_id
    product_type
    name
    dealable
    currency
    service_type
    due_diligence_label
    etf_label
    performance_by_periods
    calculated_ratios
    data(with_companies: true)
    latest_price { date value }
    product_company { id name allfunds_id }
    fund_category { id asset_class subasset_class peer_group }
    benchmark { id name isin currency }
    share_classes { id isin name currency }
    managers { id name }
  }
}
`

// GetProductDocumentsQuery retrieves documents for a fund
const GetProductDocumentsQuery = `
query GetProductDocuments($id: ID!, $pagination: Pagination!) {
  product(id: $id) {
    id
    isin
    name
    documents(pagination: $pagination) {
      results {
        id
        url
        kind
        date
        language
      }
      pagination_result { total_count page_count }
    }
  }
}
`

// GetCurrentUserQuery retrieves current user information
const GetCurrentUserQuery = `
query GetCurrentUser {
  me {
    id
    allfunds_id
    name
    email
    username
    surname
    language
    entity {
      id
      allfunds_id
      name
      icon
      kind
      premium_license
    }
  }
}
`

// GetWatchlistsQuery retrieves user watchlists
const GetWatchlistsQuery = `
query GetWatchlistsPaginated($pagination: Pagination!, $criteria: WatchlistCriteriaInput, $order: WatchlistOrder!) {
  watchlists_paginated(criteria: $criteria pagination: $pagination order: $order) {
    results {
      id
      name
      description
      total_products
      created_at
      user { id }
    }
    pagination_result { page_count total_count }
  }
}
`

// GetArticlesQuery retrieves fund insights/articles
const GetArticlesQuery = `
query GetArticles($criteria: ArticleCriteriaInput, $pagination: Pagination!, $order: ArticleOrder, $include_sponsored: Boolean) {
  articles(criteria: $criteria pagination: $pagination order: $order include_sponsored: $include_sponsored) {
    results {
      id
      title
      publish_date
      summary
      regions
      hashtags
      kinds
      entity { id name icon }
    }
    pagination_result { page_count total_count }
  }
}
`

// GetPinnedWatchlistQuery retrieves user's pinned watchlist
const GetPinnedWatchlistQuery = `
query GetPinnedWatchlist {
  pinned_watchlist { id name }
}
`

// GetWatchlistFundsQuery retrieves funds from a specific watchlist
const GetWatchlistFundsQuery = `
query GetProductsFromWatchlist($id: ID!, $pagination: Pagination!, $order: ProductOrder!, $search_string: String) {
  watchlist(id: $id) {
    id
    name
    total_products
    paginated_products(pagination: $pagination order: $order search_string: $search_string) {
      results {
        id
        product_type
        name
        isin
        currency
        performance_by_periods
        dealable
        data
        product_company { id name }
        latest_price { date value }
        fund_category { asset_class subasset_class peer_group }
        ratios: calculated_ratios
      }
      pagination_result { page_count total_count }
    }
  }
}
`

// ProductCalculationsQuery retrieves portfolio breakdown data
const ProductCalculationsQuery = `
query ProductCalculations($id: ID!, $top_size: Int, $limited: Boolean) {
  product(id: $id) {
    id
    isin
    name
    asset_type_groups
    net_asset_allocation
    net_regional_exposure
    net_currency_exposure
    net_sector_distribution
    net_rating_distribution
    net_country_exposure
    top_underlying_assets(top_size: $top_size, limited: $limited) {
      name
      weight
    }
    latest_underlying_assets_date
  }
}
`

// ProductPricesQuery retrieves NAV history (close prices, dividends, splits)
const ProductPricesQuery = `
query ProductPrices($id: ID!, $since_date: Date, $until_date: Date, $limit: Int) {
  product(id: $id) {
    id
    isin
    name
    close_prices(since_date: $since_date, until_date: $until_date, limit: $limit) {
      date
      value
    }
    dividends {
      dividend_at
      recorded_at
      payed_at
      unit
    }
    splits {
      split_at
      ratio
    }
  }
}
`

// GetSimilarFundsQuery retrieves similar/alternative funds
const GetSimilarFundsQuery = `
query GetSimilarAlternativesForProductQuery($product_id: ID!) {
  get_similar_alternatives_for_product(product_id: $product_id) {
    id
    isin
    name
    dealable
    data
    product_company { name }
    ratios: calculated_ratios
    performance_by_periods
    latest_price { date value }
  }
}
`

// LoginMutation authenticates user and returns csrf_token
const LoginMutation = `
mutation LogIn($email: String!, $password: String!) {
  log_in(email: $email, password: $password) {
    user { id }
    csrf_token
    errors
  }
}
`
