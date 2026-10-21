<?php

declare(strict_types=1);

// A WordPress request as the mu-plugin sees it: ABSPATH with dws_debugger.json, hooks, saved queries
require __DIR__ . '/hooks.php';
define('ABSPATH', $argv[1] . '/');
define('SAVEQUERIES', true);
define('WAPP_DEBUGGER_LIBRARY', dirname(__DIR__, 3) . '/vendor/autoload.php');

require dirname(__DIR__, 3) . '/integrations/wordpress/wapp-debugger.php';

// What $wpdb does after each query when SAVEQUERIES is on
$data = apply_filters('log_query_custom_data', [], 'SELECT * FROM wp_posts WHERE ID = 7', 0.0123, 'WP_Query->get_posts', microtime(true));
echo '<body>';
do_action('wp_footer');
echo '</body>';
