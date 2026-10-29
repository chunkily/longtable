// The icons a token can wear instead of art, for the table that didn't
// prepare any.
//
// Curated from Lucide, which the toolbar is already drawn with: about
// sixty that mean something at a D&D table, each with words to search by,
// since "undead" should find the skull and the ghost even though neither
// is called that. A Lucide set is thin on monsters — no dragon, spider or
// snake in this version — so a second pool from game-icons.net follows
// it: picked in game-icons.picks.json and vendored into
// game-icons.generated.ts by scripts/vendor-game-icons.mjs. Those are
// CC BY 3.0, which is why the picker credits them under its grid.
//
// `store.TokenIcons` in Go is the same list of keys and validates them,
// since a key reaches everyone's screen. TestTokenIcons_MatchTheClientList
// fails if the two drift, so a new icon goes in both, in the same order.
// Keys are namespaced (`lucide:`) so a second pool is more keys, not a
// migration.
import type { Component } from 'svelte';
import { GAME_ICONS } from './game-icons.generated';
import Axe from '@lucide/svelte/icons/axe';
import Baby from '@lucide/svelte/icons/baby';
import Beer from '@lucide/svelte/icons/beer';
import Bird from '@lucide/svelte/icons/bird';
import Bomb from '@lucide/svelte/icons/bomb';
import Bone from '@lucide/svelte/icons/bone';
import BookOpen from '@lucide/svelte/icons/book-open';
import BowArrow from '@lucide/svelte/icons/bow-arrow';
import Bug from '@lucide/svelte/icons/bug';
import Castle from '@lucide/svelte/icons/castle';
import Cat from '@lucide/svelte/icons/cat';
import Church from '@lucide/svelte/icons/church';
import CloudLightning from '@lucide/svelte/icons/cloud-lightning';
import Coins from '@lucide/svelte/icons/coins';
import Crown from '@lucide/svelte/icons/crown';
import Dog from '@lucide/svelte/icons/dog';
import DoorOpen from '@lucide/svelte/icons/door-open';
import Drama from '@lucide/svelte/icons/drama';
import Droplet from '@lucide/svelte/icons/droplet';
import Eye from '@lucide/svelte/icons/eye';
import Feather from '@lucide/svelte/icons/feather';
import Fish from '@lucide/svelte/icons/fish';
import Flag from '@lucide/svelte/icons/flag';
import Flame from '@lucide/svelte/icons/flame';
import FlaskConical from '@lucide/svelte/icons/flask-conical';
import Gem from '@lucide/svelte/icons/gem';
import Ghost from '@lucide/svelte/icons/ghost';
import Hammer from '@lucide/svelte/icons/hammer';
import Heart from '@lucide/svelte/icons/heart';
import Key from '@lucide/svelte/icons/key';
import Lock from '@lucide/svelte/icons/lock';
import Moon from '@lucide/svelte/icons/moon';
import Mountain from '@lucide/svelte/icons/mountain';
import Music from '@lucide/svelte/icons/music';
import PawPrint from '@lucide/svelte/icons/paw-print';
import Pickaxe from '@lucide/svelte/icons/pickaxe';
import Rabbit from '@lucide/svelte/icons/rabbit';
import Rat from '@lucide/svelte/icons/rat';
import ScrollText from '@lucide/svelte/icons/scroll-text';
import Shield from '@lucide/svelte/icons/shield';
import Skull from '@lucide/svelte/icons/skull';
import Snail from '@lucide/svelte/icons/snail';
import Snowflake from '@lucide/svelte/icons/snowflake';
import Sparkles from '@lucide/svelte/icons/sparkles';
import Squirrel from '@lucide/svelte/icons/squirrel';
import Star from '@lucide/svelte/icons/star';
import Sun from '@lucide/svelte/icons/sun';
import Sword from '@lucide/svelte/icons/sword';
import Swords from '@lucide/svelte/icons/swords';
import Target from '@lucide/svelte/icons/target';
import Tent from '@lucide/svelte/icons/tent';
import Tornado from '@lucide/svelte/icons/tornado';
import TreePine from '@lucide/svelte/icons/tree-pine';
import Turtle from '@lucide/svelte/icons/turtle';
import User from '@lucide/svelte/icons/user';
import Users from '@lucide/svelte/icons/users';
import VenetianMask from '@lucide/svelte/icons/venetian-mask';
import WandSparkles from '@lucide/svelte/icons/wand-sparkles';
import Worm from '@lucide/svelte/icons/worm';
import Zap from '@lucide/svelte/icons/zap';

export interface TokenIcon {
	key: string;
	/** What the picker names it. */
	label: string;
	/** What else it can be found by. Lower case. */
	words: string[];
	/** A Lucide icon's component. */
	icon?: Component;
	/** A vendored icon's SVG, drawn in currentColor. */
	svg?: { viewBox: string; body: string };
}

const LUCIDE_ICONS: TokenIcon[] = [
	{
		key: 'lucide:sword',
		label: 'Sword',
		words: ['fighter', 'warrior', 'weapon', 'blade', 'knight'],
		icon: Sword
	},
	{
		key: 'lucide:swords',
		label: 'Crossed swords',
		words: ['battle', 'fight', 'duel', 'enemy', 'bandit'],
		icon: Swords
	},
	{
		key: 'lucide:shield',
		label: 'Shield',
		words: ['guard', 'paladin', 'defender', 'soldier'],
		icon: Shield
	},
	{
		key: 'lucide:bow-arrow',
		label: 'Bow',
		words: ['ranger', 'archer', 'hunter', 'arrow'],
		icon: BowArrow
	},
	{
		key: 'lucide:axe',
		label: 'Axe',
		words: ['barbarian', 'dwarf', 'woodcutter', 'weapon'],
		icon: Axe
	},
	{
		key: 'lucide:hammer',
		label: 'Hammer',
		words: ['smith', 'cleric', 'dwarf', 'weapon'],
		icon: Hammer
	},
	{ key: 'lucide:pickaxe', label: 'Pickaxe', words: ['miner', 'dwarf', 'tool'], icon: Pickaxe },
	{
		key: 'lucide:wand-sparkles',
		label: 'Wand',
		words: ['wizard', 'sorcerer', 'mage', 'magic', 'spell'],
		icon: WandSparkles
	},
	{
		key: 'lucide:flask-conical',
		label: 'Potion',
		words: ['alchemist', 'artificer', 'flask', 'poison'],
		icon: FlaskConical
	},
	{
		key: 'lucide:scroll-text',
		label: 'Scroll',
		words: ['spell', 'quest', 'letter', 'map'],
		icon: ScrollText
	},
	{
		key: 'lucide:book-open',
		label: 'Book',
		words: ['wizard', 'spellbook', 'scholar', 'library'],
		icon: BookOpen
	},
	{
		key: 'lucide:crown',
		label: 'Crown',
		words: ['king', 'queen', 'noble', 'royal', 'boss', 'leader'],
		icon: Crown
	},
	{ key: 'lucide:gem', label: 'Gem', words: ['treasure', 'loot', 'jewel', 'crystal'], icon: Gem },
	{
		key: 'lucide:coins',
		label: 'Coins',
		words: ['treasure', 'gold', 'loot', 'merchant', 'money'],
		icon: Coins
	},
	{ key: 'lucide:key', label: 'Key', words: ['door', 'lock', 'objective', 'item'], icon: Key },
	{ key: 'lucide:lock', label: 'Lock', words: ['locked', 'door', 'chest', 'trap'], icon: Lock },
	{ key: 'lucide:door-open', label: 'Door', words: ['exit', 'entrance', 'portal'], icon: DoorOpen },
	{
		key: 'lucide:castle',
		label: 'Castle',
		words: ['keep', 'fortress', 'tower', 'location'],
		icon: Castle
	},
	{ key: 'lucide:tent', label: 'Tent', words: ['camp', 'rest', 'location'], icon: Tent },
	{
		key: 'lucide:church',
		label: 'Temple',
		words: ['church', 'shrine', 'cleric', 'priest', 'holy'],
		icon: Church
	},
	{
		key: 'lucide:skull',
		label: 'Skull',
		words: ['undead', 'skeleton', 'death', 'dead', 'lich'],
		icon: Skull
	},
	{
		key: 'lucide:bone',
		label: 'Bone',
		words: ['undead', 'skeleton', 'remains', 'dog'],
		icon: Bone
	},
	{
		key: 'lucide:ghost',
		label: 'Ghost',
		words: ['undead', 'spirit', 'spectre', 'wraith', 'haunt'],
		icon: Ghost
	},
	{
		key: 'lucide:eye',
		label: 'Eye',
		words: ['beholder', 'watcher', 'scout', 'spy', 'sentry'],
		icon: Eye
	},
	{
		key: 'lucide:drama',
		label: 'Masks',
		words: ['bard', 'actor', 'performer', 'theatre'],
		icon: Drama
	},
	{
		key: 'lucide:venetian-mask',
		label: 'Mask',
		words: ['rogue', 'thief', 'assassin', 'disguise', 'spy'],
		icon: VenetianMask
	},
	{
		key: 'lucide:heart',
		label: 'Heart',
		words: ['healer', 'cleric', 'ally', 'friend', 'love'],
		icon: Heart
	},
	{
		key: 'lucide:flame',
		label: 'Flame',
		words: ['fire', 'elemental', 'burning', 'hazard'],
		icon: Flame
	},
	{
		key: 'lucide:zap',
		label: 'Lightning',
		words: ['storm', 'elemental', 'shock', 'thunder'],
		icon: Zap
	},
	{
		key: 'lucide:snowflake',
		label: 'Snowflake',
		words: ['ice', 'cold', 'frost', 'elemental'],
		icon: Snowflake
	},
	{
		key: 'lucide:droplet',
		label: 'Droplet',
		words: ['water', 'ooze', 'slime', 'blood', 'elemental'],
		icon: Droplet
	},
	{
		key: 'lucide:cloud-lightning',
		label: 'Storm',
		words: ['weather', 'elemental', 'air', 'lightning'],
		icon: CloudLightning
	},
	{
		key: 'lucide:tornado',
		label: 'Tornado',
		words: ['air', 'wind', 'elemental', 'whirlwind', 'hazard'],
		icon: Tornado
	},
	{
		key: 'lucide:sparkles',
		label: 'Sparkles',
		words: ['magic', 'fey', 'fairy', 'spell', 'effect'],
		icon: Sparkles
	},
	{
		key: 'lucide:star',
		label: 'Star',
		words: ['celestial', 'objective', 'marker', 'important'],
		icon: Star
	},
	{ key: 'lucide:sun', label: 'Sun', words: ['light', 'celestial', 'day', 'cleric'], icon: Sun },
	{
		key: 'lucide:moon',
		label: 'Moon',
		words: ['night', 'werewolf', 'druid', 'shadow'],
		icon: Moon
	},
	{
		key: 'lucide:cat',
		label: 'Cat',
		words: ['familiar', 'beast', 'animal', 'panther', 'tiger'],
		icon: Cat
	},
	{
		key: 'lucide:dog',
		label: 'Dog',
		words: ['wolf', 'hound', 'beast', 'animal', 'companion'],
		icon: Dog
	},
	{
		key: 'lucide:bird',
		label: 'Bird',
		words: ['owl', 'hawk', 'raven', 'familiar', 'beast', 'animal'],
		icon: Bird
	},
	{
		key: 'lucide:rat',
		label: 'Rat',
		words: ['vermin', 'swarm', 'beast', 'animal', 'wererat'],
		icon: Rat
	},
	{ key: 'lucide:fish', label: 'Fish', words: ['shark', 'aquatic', 'beast', 'animal'], icon: Fish },
	{
		key: 'lucide:bug',
		label: 'Bug',
		words: ['insect', 'spider', 'beetle', 'vermin', 'swarm'],
		icon: Bug
	},
	{
		key: 'lucide:worm',
		label: 'Worm',
		words: ['purple worm', 'snake', 'serpent', 'vermin'],
		icon: Worm
	},
	{ key: 'lucide:snail', label: 'Snail', words: ['slug', 'slow', 'beast', 'animal'], icon: Snail },
	{
		key: 'lucide:turtle',
		label: 'Turtle',
		words: ['tortoise', 'dragon turtle', 'beast', 'animal'],
		icon: Turtle
	},
	{
		key: 'lucide:rabbit',
		label: 'Rabbit',
		words: ['hare', 'familiar', 'beast', 'animal'],
		icon: Rabbit
	},
	{
		key: 'lucide:squirrel',
		label: 'Squirrel',
		words: ['familiar', 'beast', 'animal', 'critter'],
		icon: Squirrel
	},
	{
		key: 'lucide:paw-print',
		label: 'Paw print',
		words: ['beast', 'animal', 'tracks', 'companion', 'wolf', 'bear'],
		icon: PawPrint
	},
	{
		key: 'lucide:feather',
		label: 'Feather',
		words: ['bird', 'harpy', 'aarakocra', 'griffon'],
		icon: Feather
	},
	{
		key: 'lucide:tree-pine',
		label: 'Tree',
		words: ['druid', 'treant', 'forest', 'plant', 'blight'],
		icon: TreePine
	},
	{
		key: 'lucide:mountain',
		label: 'Mountain',
		words: ['giant', 'earth', 'elemental', 'rock', 'golem'],
		icon: Mountain
	},
	{
		key: 'lucide:user',
		label: 'Person',
		words: ['npc', 'human', 'commoner', 'villager', 'character'],
		icon: User
	},
	{
		key: 'lucide:users',
		label: 'Group',
		words: ['crowd', 'mob', 'party', 'villagers', 'cultists'],
		icon: Users
	},
	{ key: 'lucide:baby', label: 'Child', words: ['kid', 'npc', 'halfling', 'small'], icon: Baby },
	{ key: 'lucide:music', label: 'Music', words: ['bard', 'song', 'performer'], icon: Music },
	{
		key: 'lucide:bomb',
		label: 'Bomb',
		words: ['explosive', 'trap', 'hazard', 'alchemist'],
		icon: Bomb
	},
	{
		key: 'lucide:target',
		label: 'Target',
		words: ['objective', 'marker', 'point', 'aim'],
		icon: Target
	},
	{
		key: 'lucide:flag',
		label: 'Flag',
		words: ['marker', 'objective', 'banner', 'goal'],
		icon: Flag
	},
	{
		key: 'lucide:beer',
		label: 'Tankard',
		words: ['tavern', 'innkeeper', 'drink', 'ale', 'dwarf'],
		icon: Beer
	}
];

export const TOKEN_ICONS: TokenIcon[] = [
	...LUCIDE_ICONS,
	...GAME_ICONS.map(({ key, label, words, viewBox, body }) => ({
		key,
		label,
		words,
		svg: { viewBox, body }
	}))
];

/** Whether any icon in a list needs the game-icons.net credit. */
export function needsGameIconsCredit(icons: TokenIcon[]): boolean {
	return icons.some((i) => i.key.startsWith('game-icons:'));
}

/** The icon for a key, or undefined for '' or a key that's been retired. */
export function tokenIcon(key: string | null | undefined): TokenIcon | undefined {
	return key ? TOKEN_ICONS.find((i) => i.key === key) : undefined;
}

/**
 * The icons matching a search, in list order. Matches the start of any
 * word in the label or the search words, so "und" finds undead and "wolf"
 * finds the dog and the paw print — but "ear" doesn't find "bear".
 */
export function searchTokenIcons(query: string): TokenIcon[] {
	const q = query.trim().toLowerCase();
	if (!q) return TOKEN_ICONS;
	return TOKEN_ICONS.filter((icon) =>
		[icon.label.toLowerCase(), ...icon.words].some(
			(phrase) => phrase.split(/\s+/).some((word) => word.startsWith(q)) || phrase.startsWith(q)
		)
	);
}
