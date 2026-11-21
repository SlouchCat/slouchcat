package com.slouchcat.app.mobile.shell

import com.slouchcat.app.mobile.collections.CollectionsFeature
import com.slouchcat.app.mobile.item.ItemFeature
import com.slouchcat.app.mobile.home.HomeFeature
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.mylists.MyListsFeature
import com.slouchcat.app.mobile.person.PersonFeature
import com.slouchcat.app.mobile.player.PlayerFeature
import com.slouchcat.app.mobile.playback.PlaybackSettingsFeature
import com.slouchcat.app.mobile.profilesettings.ProfileSettingsFeature
import com.slouchcat.app.mobile.search.SearchFeature

/**
 * Every feature of the phone app, one line each (concept 27, section 13: one or more per stage).
 * The order here does not matter: targets sort by `TopDestination.order`, rows by section and
 * order. A stage adds its feature here with one line `XFeature,` and nowhere else in the shell.
 * Git merges this file line by line (`merge=union` in .gitattributes): keep one feature per line,
 * each with its trailing comma, and its import on a line of its own.
 */
val features: List<Feature> = listOf(
    HomeFeature,
    MoreFeature,
    ItemFeature(),
    PersonFeature(),
    SearchFeature(),
    MyListsFeature(),
    CollectionsFeature(),
    ProfileSettingsFeature(),
    PlayerFeature(),
    PlaybackSettingsFeature(),
)
